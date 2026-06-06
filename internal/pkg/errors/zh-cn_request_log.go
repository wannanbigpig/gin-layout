package errors

func init() {
	mergeZhCNTextKey(map[string]string{
		MsgKeyRequestLogOpStatusSuccess: "成功",
		MsgKeyRequestLogOpStatusFailure: "失败",
	})
}
