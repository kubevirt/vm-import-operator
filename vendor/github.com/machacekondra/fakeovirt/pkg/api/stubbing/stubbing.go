package stubbing

type Stubbings []Stubbing

type Stubbing struct {
	Path      string
	Method    string
	Responses []RepeatedResponse
}

type RepeatedResponse struct {
	ResponseBody *string
	ResponseCode int
	Times        int
}
