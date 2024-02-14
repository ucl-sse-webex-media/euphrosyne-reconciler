from setuptools import find_packages, setup

setup(
    name="euphrosyne-recipes",
    version="0.1.0",
    packages=find_packages(),
    install_requires=[
        "requests",
        "redis",
        "tenacity",
    ],
    entry_points={
        "console_scripts": [
            "dummy = scripts.dummy:main",
            # "http-errors = scripts.http_errors:main",
            # "jira = scripts.jira:main",
        ],
    },
    extras_require={
        "dev": [
            "black",
            "codespell",
            "flake8",
            "flake8-builtins",
            "flake8-copyright",
            "isort",
            "pep8-naming",
            "pyproject-flake8",
        ],
    },
)
