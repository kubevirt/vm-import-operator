package stubbing

type StubbingBuilder struct {
	stubbings []Stubbing
}

func NewStubbingBuilder() *StubbingBuilder {
	return &StubbingBuilder{}
}

func (s *StubbingBuilder) Build() Stubbings {
	return s.stubbings
}

func (s *StubbingBuilder) Stub(stub Stubbing) *StubbingBuilder {
	s.stubbings = append(s.stubbings, stub)
	return s
}

func (s *StubbingBuilder) StubGetWithResponseCode(path string, responseCode int, body *string) *StubbingBuilder {
	return s.Stub(Stubbing{
		Path:   path,
		Method: "GET",
		Responses: []RepeatedResponse{{
			ResponseBody: body,
			ResponseCode: responseCode,
		}},
	})
}

func (s *StubbingBuilder) StubGet(path string, body *string) *StubbingBuilder {
	return s.Stub(Stubbing{
		Path:   path,
		Method: "GET",
		Responses: []RepeatedResponse{{
			ResponseBody: body,
			ResponseCode: 200,
		}},
	})
}
