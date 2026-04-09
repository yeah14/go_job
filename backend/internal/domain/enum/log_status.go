package enum

type LogStatus int8

const (
	LogStatusPending LogStatus = 0
	LogStatusRunning LogStatus = 1
	LogStatusSuccess LogStatus = 2
	LogStatusFailed  LogStatus = 3
)

func (s LogStatus) IsValid() bool {
	return s == LogStatusPending ||
		s == LogStatusRunning ||
		s == LogStatusSuccess ||
		s == LogStatusFailed
}

func (s LogStatus) String() string {
	switch s {
	case LogStatusPending:
		return "pending"
	case LogStatusRunning:
		return "running"
	case LogStatusSuccess:
		return "success"
	case LogStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

func (s LogStatus) Label() string {
	switch s {
	case LogStatusPending:
		return "待执行"
	case LogStatusRunning:
		return "执行中"
	case LogStatusSuccess:
		return "执行成功"
	case LogStatusFailed:
		return "执行失败"
	default:
		return "未知"
	}
}
