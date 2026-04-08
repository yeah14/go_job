package response

import "github.com/gin-gonic/gin"

type Body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func JSON(c *gin.Context, httpStatus int, code int, message string, data interface{}) {
	c.JSON(httpStatus, Body{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

func Success(c *gin.Context, data interface{}) {
	JSON(c, 200, 0, "success", data)
}

func Created(c *gin.Context, data interface{}) {
	JSON(c, 201, 0, "success", data)
}

func BadRequest(c *gin.Context, message string) {
	JSON(c, 400, 40000, message, nil)
}

func Unauthorized(c *gin.Context, message string) {
	JSON(c, 401, 40100, message, nil)
}

func Forbidden(c *gin.Context, message string) {
	JSON(c, 403, 40300, message, nil)
}

func NotFound(c *gin.Context, message string) {
	JSON(c, 404, 40400, message, nil)
}

func InternalError(c *gin.Context, message string) {
	JSON(c, 500, 50000, message, nil)
}
