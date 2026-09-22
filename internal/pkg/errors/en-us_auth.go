package errors

func init() {
	mergeEnUSText(map[int]string{
		UserPasswordWrong:       "Incorrect password",
		UserExists:              "User already exists",
		PhoneNumberExists:       "Phone number already exists",
		EmailExists:             "Email already exists",
		UsernameRequired:        "Username is required",
		NicknameRequired:        "Nickname is required",
		PasswordRequired:        "Password is required",
		PasswordProcessFailed:   "Password processing failed",
		SuperAdminCannotModify:  "Cannot modify default super admin password",
		SuperAdminCannotDisable: "Cannot disable default super admin",
		SuperAdminCannotDelete:  "Cannot delete default super admin",
		SamePassword:            "New password cannot be the same as current password",
		CasbinInitFailed:        "Permission initialization failed",
		TokenGenerateFailed:     "Failed to generate token",
		LoginFailed:             "Login failed, please try again later",
		CreateUserFailed:        "Failed to create user, please try again",
		UpdateUserFailed:        "Failed to update user, please try again",
		DeleteUserFailed:        "Failed to delete user, please try again",
		QueryUserDeptFailed:     "Failed to query user department association",
		SuperAdminMustKeepRole:  "Default super admin must keep super admin role",
		LoginAccountLocked:      "Account is locked, please try again later",
		RefreshTokenInvalid:     "Refresh token is invalid or expired",
		CurrentPasswordRequired: "Current password is required when changing password or login phone number",
		CurrentPasswordWrong:    "Current password is incorrect",
		SMSCredentialInvalid:    "Phone number or SMS code is incorrect",
		SMSLoginDisabled:        "SMS login is disabled",
		SMSProviderFailed:       "Failed to send SMS; check provider configuration",
	})

	mergeEnUSTextKey(map[string]string{
		MsgKeyAuthSessionExpired:        "Session expired, please log in again",
		MsgKeyAuthPermissionInitFailed:  "Permission engine initialization failed",
		MsgKeyAuthPermissionCheckFailed: "Permission check failed",
		MsgKeyAuthAPIOperationDenied:    "No permission to operate this API",
		MsgKeyAuthAccountLocked:         "Account is locked, please try again in %d minutes",
	})
}
