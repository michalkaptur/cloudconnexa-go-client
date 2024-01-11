package cloudconnexa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	userAgent = "cloudconnexa-go"
)

type Client struct {
	client *http.Client

	BaseURL     string
	Token       string
	RateLimiter *rate.Limiter

	UserAgent string

	common service

	Connectors *ConnectorsService
	DnsRecords *DNSRecordsService
	Hosts      *HostsService
	IPServices *IPServicesService
	Networks   *NetworksService
	Routes     *RoutesService
	Users      *UsersService
	UserGroups *UserGroupsService
	VPNRegions *VPNRegionsService
}

type service struct {
	client *Client
}

type Credentials struct {
	AccessToken string `json:"access_token"`
}

func NewClient(baseURL, clientId, clientSecret string) (*Client, error) {
	if clientId == "" || clientSecret == "" {
		return nil, ErrCredentialsRequired
	}

	values := map[string]string{"grant_type": "client_credentials", "scope": "default"}
	jsonData, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	tokenURL := fmt.Sprintf("%s/api/beta/oauth/token", strings.TrimRight(baseURL, "/"))
	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewBuffer(jsonData))

	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(clientId, clientSecret)
	req.Header.Add("Accept", "application/json")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("NewClient: response body: %s\n", string(body))

	var credentials Credentials
	err = json.Unmarshal(body, &credentials)
	if err != nil {
		return nil, err
	}

	c := &Client{
		client:      httpClient,
		BaseURL:     baseURL,
		Token:       credentials.AccessToken,
		UserAgent:   userAgent,
		RateLimiter: rate.NewLimiter(rate.Every(1*time.Second), 5),
	}
	c.common.client = c
	c.Connectors = (*ConnectorsService)(&c.common)
	c.DnsRecords = (*DNSRecordsService)(&c.common)
	c.Hosts = (*HostsService)(&c.common)
	c.IPServices = (*IPServicesService)(&c.common)
	c.Networks = (*NetworksService)(&c.common)
	c.Routes = (*RoutesService)(&c.common)
	c.Users = (*UsersService)(&c.common)
	c.UserGroups = (*UserGroupsService)(&c.common)
	c.VPNRegions = (*VPNRegionsService)(&c.common)
	return c, nil
}

func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	err := c.RateLimiter.Wait(context.Background())
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("status code: %d, response body: %s", res.StatusCode, string(body))
	}

	return body, nil
}
