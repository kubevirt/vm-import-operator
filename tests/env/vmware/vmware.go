package vmware

import "strings"

const (
	apiURLTemplate = "https://vcsim.@NAMESPACE:8989/sdk"
	vcsimThumbprint = "2C:11:ED:D7:13:87:7D:B5:74:18:B8:1C:42:C2:56:1F:0D:B9:5B:B9"
	vcsimUser = "user"
	vcsimPassword = "pass"
)

// Environment holds vcsim environment connection details
type Environment struct {
	ApiURL     string
	Username   string
	Password   string
	Thumbprint string
}

// NewVcsimEnvironment creates new vcsim environment
func NewVcsimEnvironment(namespace string) *Environment {
	apiURL := strings.Replace(apiURLTemplate, "@NAMESPACE", namespace, 1)
	return &Environment{
		Username:   vcsimUser,
		Password:   vcsimPassword,
		ApiURL:     apiURL,
		Thumbprint: vcsimThumbprint,
	}
}

// WithAPIURL sets the ApiURL - builder style
func (r *Environment) WithAPIURL(aPIURL string) *Environment {
	r.ApiURL = aPIURL
	return r
}

// WithUsername sets the Username - builder style
func (r *Environment) WithUsername(username string) *Environment {
	r.Username = username
	return r
}

// WithPassword sets the Password - builder style
func (r *Environment) WithPassword(password string) *Environment {
	r.Password = password
	return r
}

// WithCaCert sets the Thumbprint - builder style
func (r *Environment) WithThumbprint(thumbprint string) *Environment {
	r.Thumbprint = thumbprint
	return r
}
