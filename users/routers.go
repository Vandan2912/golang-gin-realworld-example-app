package users

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
)

func UsersRegister(router *gin.RouterGroup) {
	router.POST("", UsersRegistration)
	router.POST("/", UsersRegistration)
	router.POST("/login", UsersLogin)
}

func UserRegister(router *gin.RouterGroup) {
	router.GET("", UserRetrieve)
	router.GET("/", UserRetrieve)
	router.PUT("", UserUpdate)
	router.PUT("/", UserUpdate)
}

func ProfileRetrieveRegister(router *gin.RouterGroup) {
	router.GET("/:username", ProfileRetrieve)
}

func ProfileRegister(router *gin.RouterGroup) {
	router.POST("/:username/follow", ProfileFollow)
	router.DELETE("/:username/follow", ProfileUnfollow)
}

func ProfileRetrieve(c *gin.Context) {
	username := c.Param("username")
	userModel, err := FindOneUser(&UserModel{Username: username})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorMessage("profile", "not found"))
		return
	}
	profileSerializer := ProfileSerializer{c, userModel}
	c.JSON(http.StatusOK, gin.H{"profile": profileSerializer.Response()})
}

func ProfileFollow(c *gin.Context) {
	username := c.Param("username")
	userModel, err := FindOneUser(&UserModel{Username: username})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorMessage("profile", "not found"))
		return
	}
	myUserModel := c.MustGet("my_user_model").(UserModel)
	err = myUserModel.following(userModel)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	serializer := ProfileSerializer{c, userModel}
	c.JSON(http.StatusOK, gin.H{"profile": serializer.Response()})
}

func ProfileUnfollow(c *gin.Context) {
	username := c.Param("username")
	userModel, err := FindOneUser(&UserModel{Username: username})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewErrorMessage("profile", "not found"))
		return
	}
	myUserModel := c.MustGet("my_user_model").(UserModel)

	err = myUserModel.unFollowing(userModel)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	serializer := ProfileSerializer{c, userModel}
	c.JSON(http.StatusOK, gin.H{"profile": serializer.Response()})
}

func UsersRegistration(c *gin.Context) {
	userModelValidator := NewUserModelValidator()
	if err := userModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}

	if _, err := FindOneUser(&UserModel{Username: userModelValidator.userModel.Username}); err == nil {
		c.JSON(http.StatusConflict, common.NewErrorMessage("username", "has already been taken"))
		return
	}
	if _, err := FindOneUser(&UserModel{Email: userModelValidator.userModel.Email}); err == nil {
		c.JSON(http.StatusConflict, common.NewErrorMessage("email", "has already been taken"))
		return
	}

	if err := SaveOne(&userModelValidator.userModel); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	c.Set("my_user_model", userModelValidator.userModel)
	serializer := UserSerializer{c}
	c.JSON(http.StatusCreated, gin.H{"user": serializer.Response()})
}

func UsersLogin(c *gin.Context) {
	loginValidator := NewLoginValidator()
	if err := loginValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	userModel, err := FindOneUser(&UserModel{Email: loginValidator.userModel.Email})

	if err != nil {
		c.JSON(http.StatusUnauthorized, common.NewErrorMessage("credentials", "invalid"))
		return
	}

	if userModel.checkPassword(loginValidator.User.Password) != nil {
		c.JSON(http.StatusUnauthorized, common.NewErrorMessage("credentials", "invalid"))
		return
	}
	UpdateContextUserModel(c, userModel.ID)
	serializer := UserSerializer{c}
	c.JSON(http.StatusOK, gin.H{"user": serializer.Response()})
}

func UserRetrieve(c *gin.Context) {
	serializer := UserSerializer{c}
	c.JSON(http.StatusOK, gin.H{"user": serializer.Response()})
}

// unmarshalNullableString decodes a raw JSON value into a string pointer.
// A JSON null yields (nil, true); a JSON string yields (&value, true).
func unmarshalNullableString(raw json.RawMessage) (*string, bool) {
	var value *string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	return value, true
}

// UserUpdate handles PUT /api/user. It parses the payload as raw JSON so that
// omitted fields are preserved while explicit null clears nullable fields
// (bio, image) and is rejected for required fields (username, email, password).
func UserUpdate(c *gin.Context) {
	myUserModel := c.MustGet("my_user_model").(UserModel)
	var req struct {
		User map[string]json.RawMessage `json:"user"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewErrorMessage("body", "is invalid"))
		return
	}

	validationErrors := common.CommonError{Errors: map[string][]string{}}
	newPassword := ""

	if raw, ok := req.User["username"]; ok {
		value, valid := unmarshalNullableString(raw)
		if !valid || value == nil || *value == "" {
			validationErrors.Errors["username"] = append(validationErrors.Errors["username"], "can't be blank")
		} else {
			myUserModel.Username = *value
		}
	}
	if raw, ok := req.User["email"]; ok {
		value, valid := unmarshalNullableString(raw)
		if !valid || value == nil || *value == "" {
			validationErrors.Errors["email"] = append(validationErrors.Errors["email"], "can't be blank")
		} else {
			myUserModel.Email = *value
		}
	}
	if raw, ok := req.User["password"]; ok {
		value, valid := unmarshalNullableString(raw)
		switch {
		case !valid || value == nil || *value == "":
			validationErrors.Errors["password"] = append(validationErrors.Errors["password"], "can't be blank")
		case len(*value) < 8:
			validationErrors.Errors["password"] = append(validationErrors.Errors["password"], "is too short (minimum is 8 characters)")
		case len(*value) > 255:
			validationErrors.Errors["password"] = append(validationErrors.Errors["password"], "is too long (maximum is 255 characters)")
		default:
			newPassword = *value
		}
	}
	if raw, ok := req.User["bio"]; ok {
		value, valid := unmarshalNullableString(raw)
		if !valid {
			validationErrors.Errors["bio"] = append(validationErrors.Errors["bio"], "is invalid")
		} else if value == nil {
			myUserModel.Bio = ""
		} else {
			myUserModel.Bio = *value
		}
	}
	if raw, ok := req.User["image"]; ok {
		value, valid := unmarshalNullableString(raw)
		if !valid {
			validationErrors.Errors["image"] = append(validationErrors.Errors["image"], "is invalid")
		} else if value == nil || *value == "" {
			myUserModel.Image = nil
		} else {
			myUserModel.Image = value
		}
	}

	if len(validationErrors.Errors) > 0 {
		c.JSON(http.StatusUnprocessableEntity, validationErrors)
		return
	}
	if newPassword != "" {
		if err := myUserModel.setPassword(newPassword); err != nil {
			c.JSON(http.StatusUnprocessableEntity, common.NewError("password", err))
			return
		}
	}

	if err := SaveOne(&myUserModel); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	UpdateContextUserModel(c, myUserModel.ID)
	serializer := UserSerializer{c}
	c.JSON(http.StatusOK, gin.H{"user": serializer.Response()})
}
