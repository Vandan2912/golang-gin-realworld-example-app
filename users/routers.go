package users

import (
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

// UserUpdate handles PUT /api/user. Validation is declarative: the schema's
// binding tags run during Bind. The handler only applies the mutations —
// absent fields are preserved, null/"" clears the nullable bio and image.
func UserUpdate(c *gin.Context) {
	myUserModel := c.MustGet("my_user_model").(UserModel)
	userUpdateValidator := NewUserUpdateValidator()
	if err := userUpdateValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	if errs := userUpdateValidator.InvalidFieldErrors(); len(errs.Errors) > 0 {
		c.JSON(http.StatusUnprocessableEntity, errs)
		return
	}

	// Past validation, a Set identity field is always a valid non-blank value.
	if f := userUpdateValidator.User.Username; f.Set {
		myUserModel.Username = f.Value
	}
	if f := userUpdateValidator.User.Email; f.Set {
		myUserModel.Email = f.Value
	}
	if f := userUpdateValidator.User.Bio; f.Set {
		myUserModel.Bio = f.Value // zero value on null: clears
	}
	if f := userUpdateValidator.User.Image; f.Set {
		if !f.Valid || f.Value == "" {
			myUserModel.Image = nil
		} else {
			value := f.Value
			myUserModel.Image = &value
		}
	}
	if f := userUpdateValidator.User.Password; f.Set {
		if err := myUserModel.setPassword(f.Value); err != nil {
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
