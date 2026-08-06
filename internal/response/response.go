package response

import "net/http"

type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

func OK(c interface{ JSON(int, interface{}) }, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data, Error: nil})
}

func Created(c interface{ JSON(int, interface{}) }, data interface{}) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Data: data, Error: nil})
}

func Error(c interface{ JSON(int, interface{}) }, code int, msg string) {
	c.JSON(code, Envelope{Success: false, Data: nil, Error: msg})
}
