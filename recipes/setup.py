from setuptools import setup, find_packages

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
        ],
    },
)
