#!/bin/sh
set -e

# Cleanup
mkdir -p dist/
rm -rf dist/* || true

# PostCSS
npx postcss --env production src/style.css > dist/style.css

# Copy files
cp -v src/*.html dist/
cp -v src/*.txt dist/
