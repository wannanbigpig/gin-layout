package errors

var enUSText = map[int]string{
	SUCCESS:          "OK",
	FAILURE:          "FAIL",
	NotFound:         "resources not found",
	ServerErr:        "Internal server error",
	TooManyRequests:  "Too many requests",
	InvalidParameter: "Parameter error",
	UserDoesNotExist: "user does not exist",
	UserDisable:      "User is disabled",
	AuthorizationErr: "You have no permission",
	NotLogin:         "Please login first",
	CaptchaErr:       "Captcha error",
}
