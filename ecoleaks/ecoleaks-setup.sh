#!/bin/sh
brew install pre-commit
git config --global init.templateDir ~/.git-template
pre-commit init-templatedir ~/.git-template