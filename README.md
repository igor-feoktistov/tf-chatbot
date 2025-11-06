# tf-chatbot

`tf-chatbot` is AI chatbot to run on TrueFoundry platform.

## Configuration

`tf-chatbot` accepts the following command-line arguments:

```shell
tf-chatbot --help
  --configPath string ChatBot configuration path (default "/usr/local/etc/chatbotConfig.yaml")
  --staticPath string ChatBot static HTML directory path (default "/usr/local/etc/html")
```

### chatbotConfig.yaml
```yaml
# TF agent API endpoint
baseUrl: https://cp.tf.example.com/api/llm/agent
# HTTP session key
sessionKey: session_example
# LLM model path in TF
model: prod/us-anthropic-claude-sonnet-4-20250514-v1-0
# base64 encoded default system prompt
systemPrompt: QTx3Y<reducted>=
# Chat options
chatOptions:
  chatHistory: true
# MCP servers list
mcpServers:
    # MCP server name (mostly for better report in "about" window)
  - name: kubectl-aws-us-east-1
    # MCP server TF integration path
    integrationFqn: example:hosted-mcp-server:example-group:mcp-server:kubectl-aws-us-east-1
    enable_all_tools: false
    # list of tools to expose
    tools:
      - name: bash
      - name: kubectl
```
