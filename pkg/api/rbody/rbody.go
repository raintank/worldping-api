package rbody

import (
	"encoding/json"
	"fmt"

	"github.com/Unknwon/macaron"
	"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
)

var (
	NotFound         = ErrResp(404, fmt.Errorf("Not found"))
	PermissionDenied = ErrResp(403, fmt.Errorf("Permission Denied"))
	ServerError      = ErrResp(500, fmt.Errorf("Server error"))
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
		return ErrResp(500, err)
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

func ErrResp(code int, err error) *ApiResponse {
	resp := &ApiResponse{
		Meta: &ResponseMeta{
			Code:    code,
			Message: err.Error(),
			Type:    "error",
		},
		Body: json.RawMessage([]byte("null")),
	}
	return resp
}

func Wrap(action interface{}) macaron.Handler {
	return func(c *middleware.Context) {
		var res *ApiResponse
		val, err := c.Invoke(action)
		if err == nil && val != nil && len(val) > 0 {
			res = val[0].Interface().(*ApiResponse)
		} else {
			res = ServerError
		}

		if res.Meta.Code == 500 {
			log.Error(3, "server error: %s", res.Meta.Message)
		}

		c.JSON(200, res)
	}
}
