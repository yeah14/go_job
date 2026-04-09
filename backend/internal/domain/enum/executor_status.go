package enum

type ExecutorStatus int8

const (
	ExecutorStatusDisabled ExecutorStatus = 0
	ExecutorStatusEnabled  ExecutorStatus = 1
)

func (s ExecutorStatus) IsValid() bool {
	return s == ExecutorStatusDisabled || s == ExecutorStatusEnabled
}

func (s ExecutorStatus) String() string {
	switch s {
	case ExecutorStatusDisabled:
		return "disabled"
	case ExecutorStatusEnabled:
		return "enabled"
	default:
		return "unknown"
	}
}

func (s ExecutorStatus) Label() string {
	switch s {
	case ExecutorStatusDisabled:
		return "禁用"
	case ExecutorStatusEnabled:
		return "正常"
	default:
		return "未知"
	}
}
