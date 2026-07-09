#!/usr/bin/env python3
"""Extract OPENROUTER_API_KEY from hermes .env and write to project .env"""

hermes_env = "/home/anton/.hermes/.env"
project_env = "/home/anton/Koding/ngawur/pov-ai-indonesia/.env"

with open(hermes_env) as f:
    for line in f:
        line = line.strip()
        if line.startswith("OPENROUTER_API_KEY=") and not line.startswith("#"):
            key = line.split("=", 1)[1].strip().strip('"').strip("'")
            if key:
                with open(project_env, "w") as out:
                    out.write("OPENROUTER_API_KEY=" + key + "\n")
                    out.write("POV_AI_MODEL=openrouter/owl-alpha\n")
                    out.write("SCRAPE_INTERVAL=6\n")
                    out.write("PORT=8080\n")
                print("OK: wrote to " + project_env + " (" + str(len(key)) + " chars)")
                raise SystemExit(0)

print("ERROR: OPENROUTER_API_KEY not found")
raise SystemExit(1)
