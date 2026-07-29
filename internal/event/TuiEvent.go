package event

type other struct {
	Other string
}
type TuiEvent struct {
	SessionId     string
	ResumeSession bool
	Pwd           string
	Text          string
	Mode          string
	ThinkLevel    string
}
