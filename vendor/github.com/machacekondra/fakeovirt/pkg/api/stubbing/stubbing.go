package stubbing

type Stubbings []Stubbing

type Stubbing struct {
	Path         string
	Method       string
	ResponseBody *string
	ResponseCode int
}
