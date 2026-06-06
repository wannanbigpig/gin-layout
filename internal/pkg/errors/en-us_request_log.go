package errors

func init() {
	mergeEnUSTextKey(map[string]string{
		MsgKeyRequestLogOpStatusSuccess: "Success",
		MsgKeyRequestLogOpStatusFailure: "Failure",
	})
}
