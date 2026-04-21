#!/bin/bash

docker run -d \
  -v ollama:/home/llm/ollama/gemma4-26b/.ollama \
  -p 11434:11434 \
  --name ollama \
  ollama/ollama
