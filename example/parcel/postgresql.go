package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"github.com/jackc/pgx"
)

const (
	// DefaultPort is the default port for postgis
	DefaultPort = 5432
	// DefaultMaxConn is the max number of connections to attempt
	DefaultMaxConn = 100
	// DefaultSSLMode by default ssl is disabled
	DefaultSSLMode = "disable"
	// DefaultSSLKey by default is empty
	DefaultSSLKey = ""
	// DefaultSSLCert by default is empty
	DefaultSSLCert = ""

	AppName = "parcel"
)

// ErrInvalidSSLMode is returned when something is wrong with SSL configuration
type ErrInvalidSSLMode string

func (e ErrInvalidSSLMode) Error() string {
	return fmt.Sprintf("postgis: invalid ssl mode (%v)", string(e))
}

var PgConfig pgx.ConnPoolConfig
var PgPool *pgx.ConnPool

// ConfigTLS is used to configure TLS
// derived from github.com/jackc/pgx configTLS (https://github.com/jackc/pgx/blob/master/conn.go)
func ConfigTLS(cc *pgx.ConnConfig, sslMode string, sslKey string, sslCert string, sslRootCert string) error {

	switch sslMode {
	case "disable":
		cc.UseFallbackTLS = false
		cc.TLSConfig = nil
		cc.FallbackTLSConfig = nil
		return nil
	case "allow":
		cc.UseFallbackTLS = true
		cc.FallbackTLSConfig = &tls.Config{InsecureSkipVerify: true}
	case "prefer":
		cc.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		cc.UseFallbackTLS = true
		cc.FallbackTLSConfig = nil
	case "require":
		cc.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	case "verify-ca", "verify-full":
		cc.TLSConfig = &tls.Config{
			ServerName: cc.Host,
		}
	default:
		return ErrInvalidSSLMode(sslMode)
	}

	if sslRootCert != "" {
		caCertPool := x509.NewCertPool()

		caCert, err := ioutil.ReadFile(sslRootCert)
		if err != nil {
			return fmt.Errorf("unable to read CA file (%q): %v", sslRootCert, err)
		}

		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("unable to add CA to cert pool")
		}

		cc.TLSConfig.RootCAs = caCertPool
		cc.TLSConfig.ClientCAs = caCertPool
	}

	if (sslCert == "") != (sslKey == "") {
		return fmt.Errorf("both 'sslcert' and 'sslkey' are required")
	} else if sslCert != "" { // we must have both now
		cert, err := tls.LoadX509KeyPair(sslCert, sslKey)
		if err != nil {
			return fmt.Errorf("unable to read cert: %v", err)
		}

		cc.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	return nil
}

func NewConnection(host string, port int, db string, user string, password string) (*pgx.ConnPool, *pgx.ConnPoolConfig) {
	if port == 0 {
		port = DefaultPort
	}
	connConfig := pgx.ConnConfig{
		Host:     host,
		Port:     uint16(port),
		Database: db,
		User:     user,
		Password: password,
		LogLevel: pgx.LogLevelWarn,
		RuntimeParams: map[string]string{
			"default_transaction_read_only": "TRUE",
			"application_name":              AppName,
		},
	}
	if err := ConfigTLS(&connConfig, DefaultSSLMode, DefaultSSLKey, DefaultSSLCert, ""); err != nil {
		panic(err)
	}
	connPoolConfig := pgx.ConnPoolConfig{
		ConnConfig:     connConfig,
		MaxConnections: DefaultMaxConn,
	}
	pool, err := pgx.NewConnPool(connPoolConfig)
	if err != nil {
		panic(err)
	}
	return pool, &connPoolConfig
}
