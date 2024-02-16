# Contributing to the Euphrosyne Reconciler Recipes

Thank you for your interest in contributing to the Euphrosyne Reconciler Recipes project! This
guide will help you get started with setting up the project locally, installing the necessary
dependencies, and formatting the code.

## Setting up the project locally

After cloning the `euphrosyne-reconciler` repository and navigating to the `recipes` directory, you
can install the project locally. To install it along with the development dependencies:

```bash
# you can add the -e option to install the project in editable mode
pip install ".[dev]"
```

## Formatting the code

We use `black` and `isort` for formatting the project. The configuration for these tools can be
found in [pyproject.toml](./pyproject.toml). After installing the development dependencies you can
run the two tools to format your code:

```bash
black .
isort .
```
