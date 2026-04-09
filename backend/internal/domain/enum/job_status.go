package enum

type JobStatus int8

const (
	JobStatusPaused  JobStatus = 0
	JobStatusRunning JobStatus = 1
)

func (s JobStatus) IsValid() bool {
	return s == JobStatusPaused || s == JobStatusRunning
}

func (s JobStatus) String() string {
	switch s {
	case JobStatusPaused:
		return "paused"
	case JobStatusRunning:
		return "running"
	default:
		return "unknown"
	}
}

func (s JobStatus) Label() string {
	switch s {
	case JobStatusPaused:
		return "暂停"
	case JobStatusRunning:
		return "运行"
	default:
		return "未知"
	}
}
