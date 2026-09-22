package errors

func init() {
	mergeEnUSText(map[int]string{
		SUCCESS:                   "OK",
		FAILURE:                   "FAIL",
		NotFound:                  "resources not found",
		ServerErr:                 "Internal server error",
		TooManyRequests:           "Too many requests",
		InvalidParameter:          "Parameter error",
		UserDoesNotExist:          "user does not exist",
		UserDisable:               "User is disabled",
		ServiceDependencyNotReady: "service database is not configured, please contact the administrator",
		NoExportData:              "No data to export",
		AuthorizationErr:          "You have no permission",
		NotLogin:                  "Please login first",
		CaptchaErr:                "Captcha error",
	})
}
