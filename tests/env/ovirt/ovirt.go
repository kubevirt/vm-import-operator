package ovirt

import "strings"

const (
	apiURLTemplate = "https://imageio.@NAMESPACE:12346/ovirt-engine/api"
)

// Environment holds oVirt environment connection details
type Environment struct {
	ApiURL   string
	Username string
	Password string
	CaCert   string
}

// NewFakeOvirtEnvironment creates new fake oVirt environment
func NewFakeOvirtEnvironment(namespace string, caCert string) *Environment {
	apiURL := strings.Replace(apiURLTemplate, "@NAMESPACE", namespace, 1)
	return &Environment{
		Username: "admin@internal",
		Password: "test",
		ApiURL:   apiURL,
		CaCert:   caCert,
	}
}

func (b *Environment) WithAPIURL(aPIURL string) *Environment {
	b.ApiURL = aPIURL
	return b
}

func (b *Environment) WithUsername(username string) *Environment {
	b.Username = username
	return b
}

func (b *Environment) WithPassword(password string) *Environment {
	b.Password = password
	return b
}

func (b *Environment) WithCaCert(caCert string) *Environment {
	b.CaCert = caCert
	return b
}
