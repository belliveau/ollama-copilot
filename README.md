# Ollama Copilot


Ollama Copilot is a proxy that allows you to use Ollama as a drop-in replacement for GitHub Copilot in your editor or IDE.

**Features:**
- Proxies Copilot API requests (including HTTPS) to your local Ollama instance
- Handles completions, model listing, and more
- Secure HTTPS tunneling (CONNECT method) supported out of the box
- Logs and catches all unmatched Copilot API calls for easy debugging and extension

![Video presentation](presentation.gif)

## Installation

### Ollama

Ensure [ollama](https://ollama.com/download/linux) is installed:

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

Or follow the [manual install](https://github.com/ollama/ollama/blob/main/docs/linux.md#manual-install).

#### Models

To use the default model expected by `ollama-copilot`:

```bash
ollama pull codellama:code
```

### ollama-copilot

```bash
go install github.com/bernardo-bruning/ollama-copilot@latest
```


### Running

Ensure your `$PATH` includes `$HOME/go/bin` or `$GOPATH/bin`.

```bash
export PATH="$HOME/go/bin:$GOPATH/bin:$PATH"
```

Start the proxy:

```bash
ollama-copilot
```

Or, if you are hosting Ollama in a container or elsewhere:

```bash
OLLAMA_HOST="http://192.168.133.7:11434" ollama-copilot
```

#### HTTPS Support

Ollama Copilot supports HTTPS tunneling via the CONNECT method. No special configuration is needed for most Copilot clients.

#### Proxy Coverage & Debugging

- The proxy automatically intercepts and handles:
    - Model list requests (`/v1/models`)
    - Completions endpoints (`/v1/engines/*/completions`)
    - All HTTPS traffic to known Copilot hosts
- Any unmatched Copilot API call is logged and returns a 404, so you can monitor your logs and add support for new endpoints as needed.

## Configure IDE


### Neovim

1. Install [copilot.vim](https://github.com/github/copilot.vim)
2. Configure variables:

```vim
let g:copilot_proxy = 'http://localhost:11435'
let g:copilot_proxy_strict_ssl = v:false
```

### VS Code (Copilot Extension)

1. Install the [GitHub Copilot extension](https://marketplace.visualstudio.com/items?itemName=GitHub.copilot)
2. Open settings and add:

```json
{
    "github.copilot.advanced": {
        "debug.overrideProxyUrl": "http://localhost:11437"
    },
    "http.proxy": "http://localhost:11435",
    "http.proxyStrictSSL": false
}
```

### Zed

1. Open settings (ctrl + ,)
2. Set up edit completion proxying:

```json
{
    "features": {
        "edit_prediction_provider": "copilot"
    },
    "show_completions_on_input": true,
    "edit_predictions": {
        "copilot": {
            "proxy": "http://localhost:11435",
            "proxy_no_verify": true
        }
    }
}
```

### Emacs (copilot.el)

1. Install [copilot-emacs](https://github.com/copilot-emacs/copilot.el)
2. Configure the proxy:

```elisp
(use-package copilot
  :straight (:host github :repo "copilot-emacs/copilot.el" :files ("*.el"))
  :ensure t
  :bind (("C-<tab>" . copilot-accept-completion))
  :config
  (setq copilot-network-proxy '(:host "127.0.0.1" :port 11434 :rejectUnauthorized :json-false))
  )
```

### Curl Example

You can test the proxy directly with curl:

```bash
# List available models
curl -x http://localhost:11435 https://api.githubcopilot.com/v1/models

# Get a completion (example endpoint, adjust as needed)
curl -x http://localhost:11435 -X POST https://api.githubcopilot.com/v1/engines/copilot-codex/completions -d '{"prompt": "def hello_world():\n    "}'
```

### VScode

1. Install [copilot extension](https://marketplace.visualstudio.com/items?itemName=GitHub.copilot)
1. Sign-in or sign-up in github
1. Configure open [settings](https://code.visualstudio.com/docs/getstarted/settings) config and insert

```json
{
    "github.copilot.advanced": {
        "debug.overrideProxyUrl": "http://localhost:11437"
    },
    "http.proxy": "http://localhost:11435",
    "http.proxyStrictSSL": false
}
```

### Zed

1. [Open settings](https://zed.dev/docs/configuring-zed) (ctrl + ,)
1. Set up [edit completion proxying](https://github.com/zed-industries/zed/pull/24364):

```json
{
    "features": {
        "edit_prediction_provider": "copilot"
    },
    "show_completions_on_input": true,
    "edit_predictions": {
        "copilot": {
            "proxy": "http://localhost:11435",
            "proxy_no_verify": true
        }
    }
}
```

### Emacs

(experimental)

1. Install [copilot-emacs](https://github.com/copilot-emacs/copilot.el)
1. Configure the proxy

```elisp
(use-package copilot
  :straight (:host github :repo "copilot-emacs/copilot.el" :files ("*.el"))  ;; if you don't use "straight", install otherwise
  :ensure t
  ;; :hook (prog-mode . copilot-mode)
  :bind (
         ("C-<tab>" . copilot-accept-completion)
         )
  :config
  (setq copilot-network-proxy '(:host "127.0.0.1" :port 11434 :rejectUnauthorized :json-false))
  )
```



## Roadmap

- [x] Enable completions APIs usage; fill in the middle.
- [x] Enable flexible configuration model (Currently only supported llamacode:code).
- [x] Create self-installing functionality.
- [x] HTTPS tunneling support
- [x] Catch-all logging for unmatched Copilot API calls
- [ ] Windows setup
- [ ] Documentation on how to use.
