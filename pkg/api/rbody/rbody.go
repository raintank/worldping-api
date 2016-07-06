package rbody

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Unknwon/macaron"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
)

type ApiError struct {
	Code    int
	Message string
}

func (e ApiError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

type ApiResponse struct {
	Meta *ResponseMeta   `json:"meta"`
	Body json.RawMessage `json:"body"`
}

type ResponseMeta struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (r *ApiResponse) Error() error {
	if r.Meta.Code == 200 {
		return nil
	}
	return ApiError{Code: r.Meta.Code, Message: r.Meta.Message}
}

func OkResp(t string, body interface{}) *ApiResponse {
	bRaw, err := json.Marshal(body)
	if err != nil {
		return ErrResp(err)
	}
	resp := &ApiResponse{
		Meta: &ResponseMeta{
			Code:    200,
			Message: "success",
			Type:    t,
		},
		Body: json.RawMessage(bRaw),
	}
	return resp
}

func ErrResp(err error) *ApiResponse {
	code := 500
	message := err.Error()

	if e, ok := err.(m.AppError); ok {
		code = e.Code()
		message = e.Message()
	}

	resp := &ApiResponse{
		Meta: &ResponseMeta{
			Code:    code,
			Message: message,
			Type:    "error",
		},
		Body: json.RawMessage([]byte("null")),
	}
	return resp
}

func Wrap(action interface{}) macaron.Handler {
	return func(c *middleware.Context) {
		pre := time.Now()
		var res *ApiResponse
		val, err := c.Invoke(action)
		if err != nil {
			log.Error(3, "request handler error: %s", err.Error())
			c.JSON(500, err.Error())
		} else if val != nil && len(val) > 0 {
			res = val[0].Interface().(*ApiResponse)
		} else {
			log.Error(3, "request handler error: No response generated")
			c.JSON(500, "No response generated.")
		}

		if res.Meta.Code == 500 {
			log.Error(3, "internal server error: %s", res.Meta.Message)
		}
		timer(c, time.Since(pre))
		c.JSON(200, res)
	}
}

func timer(c *middleware.Context, duration time.Duration) {
	path := strings.Replace(strings.Trim(c.Req.URL.Path, "/"), "/", ".", -1)
	log.Debug("%s.%s took %s.", path, c.Req.Method, duration)
}
