package adminresource

type BrandingConfig struct {
	ProductName   string `json:"productName"`
	ShortName     string `json:"shortName"`
	LogoURL       string `json:"logoUrl"`
	FaviconURL    string `json:"faviconUrl"`
	PrimaryColor  string `json:"primaryColor"`
	DefaultTheme  string `json:"defaultTheme"`
	LoginTitle    string `json:"loginTitle"`
	LoginSubtitle string `json:"loginSubtitle"`
	SupportEmail  string `json:"supportEmail"`
}

func (s *Store) BrandingConfig() BrandingConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.brandingConfigLocked()
}

func (s *Store) brandingConfigLocked() BrandingConfig {
	config := defaultBrandingConfig()
	record := findRecordByCode(s.resources["settings"], "branding")
	if record == nil || record.Status == "disabled" {
		return config
	}
	applyBrandingValues(&config, record.Values)
	return config
}

func defaultBrandingConfig() BrandingConfig {
	return BrandingConfig{
		ProductName:   "Platform Go",
		ShortName:     "Platform",
		LogoURL:       "",
		FaviconURL:    "",
		PrimaryColor:  "#1677ff",
		DefaultTheme:  "tech",
		LoginTitle:    "Platform Go",
		LoginSubtitle: "Reusable operations platform foundation.",
		SupportEmail:  "",
	}
}

func applyBrandingValues(config *BrandingConfig, values map[string]string) {
	if values["productName"] != "" {
		config.ProductName = values["productName"]
	}
	if values["shortName"] != "" {
		config.ShortName = values["shortName"]
	}
	if values["logoUrl"] != "" {
		config.LogoURL = values["logoUrl"]
	}
	if values["faviconUrl"] != "" {
		config.FaviconURL = values["faviconUrl"]
	}
	if values["primaryColor"] != "" {
		config.PrimaryColor = values["primaryColor"]
	}
	if values["defaultTheme"] != "" {
		config.DefaultTheme = values["defaultTheme"]
	}
	if values["loginTitle"] != "" {
		config.LoginTitle = values["loginTitle"]
	}
	if values["loginSubtitle"] != "" {
		config.LoginSubtitle = values["loginSubtitle"]
	}
}
