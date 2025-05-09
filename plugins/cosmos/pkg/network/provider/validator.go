package provider

import (
	"errors"
	"net"
	"strings"

	crossplanev1beta1 "github.com/overlock-network/api/go/node/overlock/crossplane/v1beta1"
)

func ValidateProvider(p *crossplanev1beta1.MsgCreateProvider) error {
	if p.Metadata == nil || strings.TrimSpace(p.Metadata.Name) == "" {
		return errors.New("provider metadata name is required")
	}

	if net.ParseIP(p.Ip) == nil {
		return errors.New("invalid IP address")
	}

	if p.Port == 0 || p.Port > 65535 {
		return errors.New("invalid port number")
	}

	if len(p.CountryCode) != 2 {
		return errors.New("country code must be 2 characters")
	}

	validEnvTypes := map[string]bool{
		"crossplane": true,
		"argocd":     true,
	}
	if !validEnvTypes[strings.ToLower(p.EnvironmentType)] {
		return errors.New("environment type must be either 'crossplane' or 'argocd'")
	}

	validAvailability := map[string]bool{
		"available":   true,
		"unavailable": true,
		"maintenance": true,
	}
	if !validAvailability[strings.ToLower(p.Availability)] {
		return errors.New("invalid availability status")
	}

	return nil
}
