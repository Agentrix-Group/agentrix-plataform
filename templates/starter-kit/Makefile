.PHONY: help setup eval train pack clean

help:
	@echo "=================================================================="
	@echo "Agentrix AI Starter Kit Commands"
	@echo "=================================================================="
	@echo "  make setup   - Create Python venv, install SDK and requirements"
	@echo "  make eval    - Evaluate agents against 5 official baseline bots"
	@echo "  make train   - Train neural policy on engine data into my_bot"
	@echo "  make pack    - Validate 3 admission ticks and build my_bot.zip"
	@echo "  make clean   - Clean cache files"
	@echo "=================================================================="

setup:
	python3 -m venv .venv
	. .venv/bin/activate && pip install --upgrade pip
	. .venv/bin/activate && pip install -r requirements.txt
	. .venv/bin/activate && pip install -e sdk/
	chmod +x bin/agentrix-arbiter
	@echo "\n[OK] Setup complete! Activate your virtualenv with:\n  source .venv/bin/activate\n"

eval:
	agentrix-eval --policies hunter,mob_farmer,survivor,zone_controller,random --seeds 10

train:
	agentrix-train --matches 15 --epochs 40 --output-dir my_bot

pack:
	agentrix-pack --bot-dir my_bot --output my_bot.zip

clean:
	find . -type d -name "__pycache__" -exec rm -rf {} +
	rm -f my_bot.zip
