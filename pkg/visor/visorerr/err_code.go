package visorerr

// ErrCode is to be returned to outer code in
// case this code doesn't support Go errors.
type ErrCode int

const (
	ErrCodeNoError ErrCode = iota
	ErrCodeInvalidPK
	ErrCodeInvalidVisorConfig
	ErrCodeInvalidAddrResolverURL
	ErrCodeSTCPInitFailed
	ErrCodeSTCPRInitFailed
	ErrCodeSUDPHInitFailed
	ErrCodeDmsgListenFailed
	ErrCodeTpDiscUnavailable
	ErrCodeFailedToStartRouter
	ErrCodeFailedToSetupHVGateway

	ErrCodeUnknown ErrCode = 999
)
