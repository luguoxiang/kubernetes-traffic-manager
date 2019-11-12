package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"
)

type SecretManager struct {
	RootCertificate *x509.Certificate
	RootKey         *rsa.PrivateKey

	PemRootCertBytes []byte
	PemRootKeyBytes  []byte
}

func NewSecretManager() (*SecretManager, error) {
	subject := pkix.Name{
		Organization: []string{"Simplistio"},
		Country:      []string{"US"},
	}
	template := &x509.Certificate{
		Issuer:                subject,
		Subject:               subject,
		SerialNumber:          big.NewInt(2),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	ca_b, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(ca_b)
	if err != nil {
		return nil, err
	}
	return &SecretManager{
		RootCertificate:  caCert,
		RootKey:          priv,
		PemRootCertBytes: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca_b}),
		PemRootKeyBytes: pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		}),
	}, nil

}

func (manager *SecretManager) GenerateGatewaySecret(hosts []string) ([]byte, []byte, error) {

	subject := pkix.Name{
		Organization: []string{"httpbin"},
		Country:      []string{"US"},
	}
	keyBytes, _ := rsa.GenerateKey(rand.Reader, 2048)

	crTemplate := x509.CertificateRequest{
		Subject:            subject,
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	csrBytes, _ := x509.CreateCertificateRequest(rand.Reader, &crTemplate, keyBytes)

	clientCSR, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return nil, nil, err
	}

	// create client certificate template
	template := x509.Certificate{
		Signature:          clientCSR.Signature,
		SignatureAlgorithm: clientCSR.SignatureAlgorithm,

		PublicKeyAlgorithm: clientCSR.PublicKeyAlgorithm,
		PublicKey:          clientCSR.PublicKey,

		SerialNumber:          big.NewInt(3),
		Issuer:                manager.RootCertificate.Subject,
		Subject:               subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  false,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}
	// create client certificate from template and CA public key
	ca_b, err := x509.CreateCertificate(rand.Reader, &template, manager.RootCertificate, clientCSR.PublicKey, manager.RootKey)
	if err != nil {
		return nil, nil, err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca_b}),
		pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(keyBytes),
		}), nil

}
