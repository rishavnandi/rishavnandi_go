package main

// Site content that used to live in src/data/*.ts and src/components/*.svelte.

const (
	siteURL   = "https://www.rishavnandi.com"
	siteName  = "Rishav Nandi"
	siteTitle = siteName + " | Gen AI Platform Engineer & DevOps"
	desc      = siteName + " is a Gen AI Platform Engineer and DevOps practitioner building AI infrastructure, automation, and self-hosted systems."
	socialImg = siteURL + "/images/readme_img.png"
)

type experience struct {
	Role, Company, CompanyURL, Start, End, About string
}

var experiences = []experience{
	{
		Role: "Gen AI Platform Engineer",
		About: "Gen AI platform engineer building LLM data pipelines, SLM fine-tuning, agentic assistants, " +
			"and distributed data services, including Ray workloads that improved processing by up to 30x. " +
			"Served as a Forward Deployed Engineer on a Palantir-based LLM data pipeline for a major US " +
			"healthcare company. Separately built GitLab review bots and a LangGraph, Vertex AI, RAG, and MCP " +
			"assistant, managed Docker and Kubernetes deployments, and adopted uv and Bun to cut CI/CD " +
			"execution time by up to 50%.",
		Company: "Tata Consultancy Services", CompanyURL: "https://tcs.com/",
		Start: "Jul 2025", End: "Present",
	},
	{
		Role: "Open-Source Maintainer",
		About: "Maintain infrastructure and automation projects across Ansible, Docker, Terraform, and " +
			"shell scripting, with more than 530 combined GitHub stars.",
		Company: "GitHub", CompanyURL: "https://github.com/rishavnandi",
		Start: "Aug 2021", End: "Present",
	},
	{
		Role: "Machine Learning Co-Lead",
		About: "Ran machine learning boot camps and workshops for more than 200 students, covering " +
			"exploratory data analysis, model training, and deployment.",
		Company:    "Google Developer Club Bhubaneswar",
		CompanyURL: "https://developers.google.com/community/gdg",
		Start:      "Aug 2022", End: "Aug 2023",
	},
}

type project struct {
	Title, Description, URL, GitHubURL, Icon string
	Tags                                     []string
	Badge                                    string // "latest" | "updated" | ""
}

var featuredProjects = []project{
	{
		Title:       "Kusama",
		Description: "Agent-native data and machine-learning studio for datasets, training, and model serving",
		Tags:        []string{"Docker", "Next.js", "Git"},
		Badge:       "latest",
		URL:         "https://app.kusama.autos",
		GitHubURL:   "https://github.com/rishavnandi/kusama",
		Icon:        "/images/kusama_logo.svg",
	},
	{
		Title:       "TSDeck",
		Description: "Self-hosted app catalog that generates one-line Docker and Tailscale setup commands",
		Tags:        []string{"Hono", "Docker", "Git"},
		Badge:       "latest",
		URL:         "https://tsdeck.rishavnandi.workers.dev",
		GitHubURL:   "https://github.com/rishavnandi/tsdeck",
		Icon:        "/images/tailscale_logo.svg",
	},
	{
		Title:       "Ansible Homelab",
		Description: "Ansible playbooks for deploying Docker homelab services",
		Tags:        []string{"Ansible", "Docker", "Git", "Linux"},
		Badge:       "updated",
		GitHubURL:   "https://github.com/rishavnandi/ansible_homelab",
	},
	{
		Title:       "Docker Compose Boilerplates",
		Description: "Reusable Docker Compose templates for self-hosted services",
		Tags:        []string{"Docker", "Git", "Linux"},
		GitHubURL:   "https://github.com/rishavnandi/boiler_plates",
	},
}

type repo struct {
	Name, Description, HTMLURL, Language string
	Topics                               []string
	Stars                                int
}

type socialLink struct{ Name, URL, Icon string }

// Social links, in header order.
var socials = []socialLink{
	{"LinkedIn", "https://www.linkedin.com/in/rishavnandi/", "linkedin"},
	{"Twitter", "https://twitter.com/rishav__nandi", "x"},
	{"GitHub", "https://github.com/rishavnandi", "github"},
}
