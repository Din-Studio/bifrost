package lib

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// This file holds the typed, file-sourced identity configuration for the OSS
// build: the `scim_config` block that the enterprise schema already defines and
// the OSS-owned `identity_sync` block. Phase 1 of the OSS identity work only
// parses and validates these sections at startup; nothing here is persisted to
// the config store, exposed over the API, or hot-reloaded yet. See
// docs/architecture/identity/overview.mdx for the phase plan.

// ScimProviderGeneric is the only scim_config.provider value the OSS build
// implements. Every other enum value in the schema is enterprise-only and is
// rejected at load time instead of being silently ignored.
const ScimProviderGeneric = "generic"

// ScimConfig mirrors $defs.scim_config. Config is kept raw because its shape
// depends on Provider; use Generic() to decode it for the generic provider.
type ScimConfig struct {
	Enabled         bool                 `json:"enabled"`
	Provider        string               `json:"provider,omitempty"`
	Config          json.RawMessage      `json:"config,omitempty"`
	TrustedNetworks []ScimTrustedNetwork `json:"trusted_networks,omitempty"`

	generic *GenericOIDCConfig
}

// ScimTrustedNetwork mirrors one item of $defs.scim_config.trusted_networks.
type ScimTrustedNetwork struct {
	Cidr        string `json:"cidr"`
	Description string `json:"description,omitempty"`
}

// GenericOIDCConfig mirrors $defs.generic_config field for field (camelCase
// JSON tags are the enterprise wire contract and must not be renamed).
type GenericOIDCConfig struct {
	IssuerURL                      string                              `json:"issuerUrl"`
	ClientID                       string                              `json:"clientId"`
	ClientSecret                   *schemas.SecretVar                  `json:"clientSecret,omitempty"`
	Audience                       string                              `json:"audience,omitempty"`
	AuthorizationEndpoint          string                              `json:"authorizationEndpoint,omitempty"`
	TokenEndpoint                  string                              `json:"tokenEndpoint,omitempty"`
	UserinfoEndpoint               string                              `json:"userinfoEndpoint,omitempty"`
	TeamIDsField                   string                              `json:"teamIdsField,omitempty"`
	RolesField                     string                              `json:"rolesField,omitempty"`
	Scopes                         []string                            `json:"scopes,omitempty"`
	AttributeRoleMappings          []ScimAttributeRoleMapping          `json:"attributeRoleMappings,omitempty"`
	RoleResolutionStrategy         string                              `json:"roleResolutionStrategy,omitempty"`
	ClaimsSyncMode                 string                              `json:"claimsSyncMode,omitempty"`
	AttributeTeamMappings          []ScimAttributeTeamMapping          `json:"attributeTeamMappings,omitempty"`
	AttributeBusinessUnitMappings  []ScimAttributeBusinessUnitMapping  `json:"attributeBusinessUnitMappings,omitempty"`
	AttributeAccessProfileMappings []ScimAttributeAccessProfileMapping `json:"attributeAccessProfileMappings,omitempty"`
	AuthProxy                      *ScimAuthProxyConfig                `json:"authProxy,omitempty"`
	ClaimScimAttributes            map[string]ScimClaimAttribute       `json:"claimScimAttributes,omitempty"`
	ProvisioningToken              *schemas.SecretVar                  `json:"provisioningToken,omitempty"`
}

// ScimAttributeRoleMapping mirrors one item of $defs.scim_attribute_role_mappings.
type ScimAttributeRoleMapping struct {
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
	Role      string `json:"role"`
}

// ScimAttributeTeamMapping mirrors one item of $defs.scim_attribute_team_mappings.
type ScimAttributeTeamMapping struct {
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
	Team      string `json:"team,omitempty"`
}

// ScimAttributeBusinessUnitMapping mirrors one item of
// $defs.scim_attribute_business_unit_mappings. Business units are enterprise
// only; the field is typed so the schema stays in sync, but OSS ignores it.
type ScimAttributeBusinessUnitMapping struct {
	Attribute    string `json:"attribute"`
	Value        string `json:"value"`
	BusinessUnit string `json:"business_unit,omitempty"`
}

// ScimAttributeAccessProfileMapping mirrors one item of
// $defs.scim_attribute_access_profile_mappings. Access profiles are enterprise
// only; the field is typed so the schema stays in sync, but OSS ignores it.
type ScimAttributeAccessProfileMapping struct {
	Attribute     string `json:"attribute"`
	Value         string `json:"value"`
	AccessProfile string `json:"accessProfile"`
}

// ScimAuthProxyConfig mirrors $defs.scim_auth_proxy.
type ScimAuthProxyConfig struct {
	Enabled          bool     `json:"enabled"`
	Provider         string   `json:"provider,omitempty"`
	Mode             string   `json:"mode,omitempty"`
	HeaderName       string   `json:"headerName,omitempty"`
	IssuerURL        string   `json:"issuerUrl,omitempty"`
	JwksURL          string   `json:"jwksUrl,omitempty"`
	Audience         string   `json:"audience,omitempty"`
	AllowedAudiences []string `json:"allowedAudiences,omitempty"`
	Region           string   `json:"region,omitempty"`
	ExpectedSigner   string   `json:"expectedSigner,omitempty"`
	PublicKeyBaseURL string   `json:"publicKeyBaseUrl,omitempty"`
	UserIDClaim      string   `json:"userIdClaim,omitempty"`
}

// ScimClaimAttribute mirrors one value of $defs.scim_claim_attributes.
type ScimClaimAttribute struct {
	AttributeType  string `json:"attributeType"`
	AttributeValue string `json:"attributeValue"`
}

// Generic decodes Config as $defs.generic_config. It is only valid when
// Provider is "generic"; any other provider returns an error because the OSS
// build has no type for it.
func (s *ScimConfig) Generic() (*GenericOIDCConfig, error) {
	if s == nil {
		return nil, fmt.Errorf("scim_config: section is absent")
	}
	if s.generic != nil {
		return s.generic, nil
	}
	if s.Provider != ScimProviderGeneric {
		return nil, fmt.Errorf("scim_config: provider %q is not supported in the OSS build (only %q)", s.Provider, ScimProviderGeneric)
	}
	if len(s.Config) == 0 || string(s.Config) == "null" {
		return nil, fmt.Errorf("scim_config.config: required for provider %q", ScimProviderGeneric)
	}
	var generic GenericOIDCConfig
	if err := json.Unmarshal(s.Config, &generic); err != nil {
		return nil, fmt.Errorf("scim_config.config: %w", err)
	}
	s.generic = &generic
	return s.generic, nil
}

// IdentitySyncConfig mirrors $defs.identity_sync (OSS-owned).
type IdentitySyncConfig struct {
	Casdoor                   *CasdoorSyncConfig `json:"casdoor,omitempty"`
	AllowInsecureIssuerForDev bool               `json:"allow_insecure_issuer_for_dev,omitempty"`
	LinkExternalIDToIssuer    string             `json:"link_external_id_to_issuer,omitempty"`
}

// CasdoorSyncConfig mirrors $defs.identity_sync_casdoor.
type CasdoorSyncConfig struct {
	Enabled             bool               `json:"enabled"`
	Organization        string             `json:"organization"`
	WebhookSecret       *schemas.SecretVar `json:"webhook_secret,omitempty"`
	ScanInterval        string             `json:"scan_interval,omitempty"`
	InventoryInterval   string             `json:"inventory_interval,omitempty"`
	PageSize            int                `json:"page_size,omitempty"`
	AllowedWebhookCIDRs []string           `json:"allowed_webhook_cidrs,omitempty"`
}

const (
	casdoorDefaultScanInterval      = 15 * time.Minute
	casdoorDefaultInventoryInterval = 24 * time.Hour
	casdoorDefaultPageSize          = 100
	casdoorMaxPageSize              = 1000
)

// ScanIntervalDuration returns scan_interval with the schema default applied.
// Validation has already rejected unparsable values at load time.
func (c *CasdoorSyncConfig) ScanIntervalDuration() time.Duration {
	return durationOrDefault(c.ScanInterval, casdoorDefaultScanInterval)
}

// InventoryIntervalDuration returns inventory_interval with the schema default applied.
func (c *CasdoorSyncConfig) InventoryIntervalDuration() time.Duration {
	return durationOrDefault(c.InventoryInterval, casdoorDefaultInventoryInterval)
}

// EffectivePageSize returns page_size with the schema default applied.
func (c *CasdoorSyncConfig) EffectivePageSize() int {
	if c.PageSize <= 0 {
		return casdoorDefaultPageSize
	}
	return c.PageSize
}

func durationOrDefault(raw string, fallback time.Duration) time.Duration {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

// loadScimConfig validates the file-sourced scim_config section and publishes
// it on Config. An absent or disabled section is a no-op, which is what keeps
// existing deployments byte-for-byte unchanged. An enabled section must use the
// generic provider: the OSS build has no implementation for the enterprise
// providers, and silently ignoring them would let an operator believe okta or
// entra SSO is in effect when it is not.
func loadScimConfig(_ context.Context, config *Config, configData *ConfigData) error {
	scim := configData.ScimConfig
	if scim == nil || !scim.Enabled {
		return nil
	}
	if scim.Provider == "" {
		return fmt.Errorf("scim_config.provider: required when scim_config.enabled is true")
	}
	if scim.Provider != ScimProviderGeneric {
		return fmt.Errorf("scim_config: provider %q is not supported in the OSS build (only %q)", scim.Provider, ScimProviderGeneric)
	}
	generic, err := scim.Generic()
	if err != nil {
		return err
	}
	if strings.TrimSpace(generic.IssuerURL) == "" {
		return fmt.Errorf("scim_config.config.issuerUrl: required")
	}
	if strings.TrimSpace(generic.ClientID) == "" {
		return fmt.Errorf("scim_config.config.clientId: required")
	}
	issuer, err := url.Parse(generic.IssuerURL)
	if err != nil || issuer.Host == "" {
		return fmt.Errorf("scim_config.config.issuerUrl: %q is not an absolute URL", generic.IssuerURL)
	}
	allowInsecure := configData.IdentitySync != nil && configData.IdentitySync.AllowInsecureIssuerForDev
	switch issuer.Scheme {
	case "https":
	case "http":
		if !allowInsecure {
			return fmt.Errorf("scim_config.config.issuerUrl: %q must use https (set identity_sync.allow_insecure_issuer_for_dev=true for local development only)", generic.IssuerURL)
		}
		logger.Warn("scim_config.config.issuerUrl %q uses plain http; identity_sync.allow_insecure_issuer_for_dev is set, which is only acceptable for local development", generic.IssuerURL)
	default:
		return fmt.Errorf("scim_config.config.issuerUrl: %q must use https", generic.IssuerURL)
	}
	for i, tn := range scim.TrustedNetworks {
		if err := validateIPOrCIDR(tn.Cidr); err != nil {
			return fmt.Errorf("scim_config.trusted_networks[%d].cidr: %w", i, err)
		}
	}
	config.ScimConfig = scim
	return nil
}

// loadIdentitySyncConfig validates the OSS-owned identity_sync section and
// publishes it on Config. The Casdoor bridge can only be enabled on top of an
// enabled generic scim_config pointing at the same Casdoor instance, because it
// reuses that application's clientId/clientSecret for directory API calls.
func loadIdentitySyncConfig(_ context.Context, config *Config, configData *ConfigData) error {
	sync := configData.IdentitySync
	if sync == nil {
		return nil
	}
	if sync.LinkExternalIDToIssuer != "" {
		u, err := url.Parse(sync.LinkExternalIDToIssuer)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return fmt.Errorf("identity_sync.link_external_id_to_issuer: %q is not an absolute http(s) URL", sync.LinkExternalIDToIssuer)
		}
	}
	if casdoor := sync.Casdoor; casdoor != nil && casdoor.Enabled {
		if config.ScimConfig == nil {
			return fmt.Errorf("identity_sync.casdoor.enabled: requires scim_config.enabled=true with provider %q", ScimProviderGeneric)
		}
		if strings.TrimSpace(casdoor.Organization) == "" {
			return fmt.Errorf("identity_sync.casdoor.organization: required")
		}
		if err := validateOptionalDuration(casdoor.ScanInterval); err != nil {
			return fmt.Errorf("identity_sync.casdoor.scan_interval: %w", err)
		}
		if err := validateOptionalDuration(casdoor.InventoryInterval); err != nil {
			return fmt.Errorf("identity_sync.casdoor.inventory_interval: %w", err)
		}
		if casdoor.PageSize < 0 || casdoor.PageSize > casdoorMaxPageSize {
			return fmt.Errorf("identity_sync.casdoor.page_size: %d is out of range [1, %d]", casdoor.PageSize, casdoorMaxPageSize)
		}
		for i, cidr := range casdoor.AllowedWebhookCIDRs {
			if err := validateIPOrCIDR(cidr); err != nil {
				return fmt.Errorf("identity_sync.casdoor.allowed_webhook_cidrs[%d]: %w", i, err)
			}
		}
	}
	config.IdentitySync = sync
	return nil
}

func validateOptionalDuration(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("%q is not a Go duration (e.g. \"15m\")", raw)
	}
	if d <= 0 {
		return fmt.Errorf("%q must be positive", raw)
	}
	return nil
}

func validateIPOrCIDR(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("empty entry")
	}
	if strings.Contains(value, "/") {
		if _, _, err := net.ParseCIDR(value); err != nil {
			return fmt.Errorf("%q is not a valid CIDR", value)
		}
		return nil
	}
	if net.ParseIP(value) == nil {
		return fmt.Errorf("%q is not a valid IP or CIDR", value)
	}
	return nil
}
