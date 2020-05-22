package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	api "github.com/machacekondra/fakeovirt/pkg/api/stubbing"
)

type FakeOvirtClient struct {
	client  *http.Client
	baseUrl string
}

func NewFakeOvirtClient(baseUrl string, tlsConfig *tls.Config) FakeOvirtClient {
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{Transport: tr}
	return FakeOvirtClient{
		client:  client,
		baseUrl: baseUrl,
	}
}

func NewInsecureFakeOvirtClient(baseUrl string) FakeOvirtClient {
	return NewFakeOvirtClient(baseUrl, &tls.Config{InsecureSkipVerify: true})
}

func (c *FakeOvirtClient) Reset(configurators ...string) error {
	var query string
	if len(configurators) > 0 {
		value := strings.Join(configurators, ",")
		query = "?configurators=" + value
	}

	response, err := c.client.Post(c.baseUrl+"/reset"+query, "plain/text", bytes.NewBufferString(""))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("reset request failed: %s", response.Status)
	}
	return nil
}

func (c *FakeOvirtClient) Stub(stubbings api.Stubbings) error {
	payload, err := json.Marshal(stubbings)
	if err != nil {
		return err
	}

	response, err := c.client.Post(c.baseUrl+"/stub", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("stubbing request failed: %s", response.Status)
	}
	return nil
}
