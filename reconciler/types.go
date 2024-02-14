package main

type IncidentBotMessage struct {
	UUID     string   `json:"uuid"`
	Actions  []Action `json:"actions"`
	Analysis string   `json:"analysis"`
}

type Recipe struct {
	Config    *RecipeConfig `json:"config,omitempty"`
	Execution *struct {
		Name     string `json:"name"`
		Incident string `json:"incident"`
		Status   string `json:"status"`
		Results  struct {
			Analysis string   `json:"analysis"`
			JSON     string   `json:"json"`
			Links    []string `json:"links"`
		} `json:"results"`
	} `json:"execution,omitempty"`
}

type RecipeConfig struct {
	Image      string `yaml:"image"`
	Entrypoint string `yaml:"entrypoint"`
	Params     []struct {
		Name  string `yaml:"name"`
		Value string `yaml:"value"`
	} `yaml:"params"`
}

type Action struct {
	Action      string `json:"action"`
	Description string `json:"description"`
}
