import os
import re

path = "internal/config/config_test.go"
with open(path, "r") as f:
    content = f.read()

# Replace block yaml
content = re.sub(r'ntfy_topic: (.*)', r'ntfy:\n    topic: \1', content)
content = re.sub(r'ntfy_server: (.*)', r'  server: \1', content)
content = re.sub(r'ntfy_token: (.*)', r'  token: \1', content)

# Replace inline yaml in strings
content = content.replace(r'notify:\n  ntfy_topic: "', r'notify:\n  ntfy:\n    topic: "')
content = content.replace(r'notify:\n  ntfy:\n    topic: "', r'notify:\n  ntfy:\n    topic: "') # Just in case
content = content.replace(r'  ntfy_server: \""+server', r'    server: \""+server')

# Replace struct fields
content = content.replace("cfg.Notify.NtfyTopic", "cfg.Notify.Ntfy.Topic")
content = content.replace("cfg.Notify.NtfyServer", "cfg.Notify.Ntfy.Server")

with open(path, "w") as f:
    f.write(content)
