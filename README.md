<p align="center">
  <img alt="Terragrunt Atlantis Config by Transcend" src="https://user-images.githubusercontent.com/7354176/78756035-f9863480-792e-11ea-96d3-d4ffe50e0269.png"/>
</p>
<h1 align="center">Terragrunt Atlantis Config</h1>
<p align="center">
  <strong>Generate Atlantis Config for Terragrunt projects.</strong>
</p>
<br />

## What is this?

[Atlantis](runatlantis.io) is an awesome tool for Terraform pull request automation. Each repo can have a YAML config file that defines Terraform module dependendcies, so that PRs that affect dependent modules will automatically generate `terraform plan`s for those modules.

[Terragrunt](https://terragrunt.gruntwork.io) is a Terraform wrapper, which has the concept of dependencies built into its configuration.

This tool creates Atlantis YAML configurations for Terragrunt projects by:

- Finding all `terragrunt.hcl` in a repo
- Evaluating their "dependency" and "terraform" source blocks to find their dependencies
- Creating a Directed Acyclic Graph of all dependencies
- Constructing and logging YAML in Atlantis' config spec that reflects the graph

This is especially useful for organizations that use monorepos for their Terragrunt config (as we do at Transcend), and have thousands of lines of config.

## Installation and Usage

Recommended: Install any version via go get:

```bash
cd && GO111MODULE=on go get github.com/transcend-io/terragrunt-atlantis-config@master && cd -
```

Alternative: Install a stable versions via Homebrew:

```bash
brew install transcend-io/tap/terragrunt-atlantis-config
```

Usage:

```bash
# From the root of your repo
terragrunt-atlantis-config generate

# or from anywhere
terragrunt-atlantis-config generate --root /some/path/to/your/repo/root

# output to a file
terragrunt-atlantis-config generate --autoplan --output ./atlantis.yaml

# enable auto plan
terragrunt-atlantis-config generate --autoplan

# define the workflow
terragrunt-atlantis-config generate --workflow web --output ./atlantis.yaml

# ignore parent terragrunt configs (those which don't reference a terraform module)
terragrunt-atlantis-config generate --ignore-parent-terragrunt
```

Finally, check the log output (or your output file) for the YAML.

## Contributing

To test any changes you've made, run `make test`.

Once all your changes are passing and your PR is reviewed, a merge into `master` will trigger a CircleCI job to build the new binary, test it, and deploy it's artifacts to an S3 bucket.

You can then open a PR on our homebrew tap similar to https://github.com/transcend-io/homebrew-tap/pull/4, and as soon as that merges your code will be released.

## License

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Ftranscend-io%2Fterragrunt-atlantis-config.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Ftranscend-io%2Fterragrunt-atlantis-config?ref=badge_large)
