package driver

import (
	"context"
	"database/sql/driver"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
)

type bigQueryDriver struct {
}

type bigQueryConfig struct {
	projectID      string
	location       string
	dataSet        string
	scopes         []string
	endpoint       string
	disableAuth    bool
	credentialFile string
	credentialJSON string
}

func (b bigQueryDriver) Open(uri string) (driver.Conn, error) {
	if uri == "scanner" {
		return &scannerConnection{}, nil
	}

	config, err := configFromUri(uri)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	opts := []option.ClientOption{option.WithScopes(config.scopes...)}
	if config.endpoint != "" {
		opts = append(opts, option.WithEndpoint(config.endpoint))
	}
	if config.disableAuth {
		opts = append(opts, option.WithoutAuthentication())
	}
	if config.credentialFile != "" {
		opts = append(opts, option.WithCredentialsFile(config.credentialFile))
	}
	if config.credentialJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(config.credentialJSON)))
	}

	client, err := bigquery.NewClient(ctx, config.projectID, opts...)
	if err != nil {
		return nil, err
	}

	return &bigQueryConnection{
		ctx:    ctx,
		client: client,
		config: *config,
	}, nil
}

func configFromUri(uri string) (*bigQueryConfig, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, invalidConnectionStringError(uri)
	}

	if u.Scheme != "bigquery" {
		return nil, fmt.Errorf("invalid prefix, expected bigquery:// got: %s", uri)
	}

	if u.Path == "" {
		return nil, invalidConnectionStringError(uri)
	}

	fields := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(fields) > 2 {
		return nil, invalidConnectionStringError(uri)
	}

	config := &bigQueryConfig{
		projectID:      u.Hostname(),
		dataSet:        fields[len(fields)-1],
		scopes:         getScopes(u.Query()),
		endpoint:       u.Query().Get("endpoint"),
		disableAuth:    u.Query().Get("disable_auth") == "true",
		credentialFile: u.Query().Get("credential_file"),
	}

	if u.Query().Get("credential_json") != nil {
		credentialsJSON, err := base64.StdEncoding.DecodeString(u.Query().Get("credential_json"))
		if err != nil {
			return nil, err
		} else {
			config.credentialJSON = credentialsJSON
		}
	}

	if len(fields) == 2 {
		config.location = fields[0]
	}

	return config, nil
}

func getScopes(query url.Values) []string {
	q := strings.Trim(query.Get("scopes"), ",")
	if q == "" {
		return []string{}
	}
	return strings.Split(q, ",")
}

func invalidConnectionStringError(uri string) error {
	return fmt.Errorf("invalid connection string: %s", uri)
}
