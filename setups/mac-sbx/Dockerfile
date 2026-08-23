FROM docker/sandbox-templates:claude-code

USER root

# Basic tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    zsh \
    curl \
    jq \
    make \
    vim \
    git \
    && rm -rf /var/lib/apt/lists/*

# Set zsh as default shell
RUN chsh -s /usr/bin/zsh agent

USER agent
