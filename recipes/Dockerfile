FROM python:3.11
WORKDIR /recipes
COPY . /recipes

RUN pip install .

EXPOSE 80
CMD ["dummy", "--data", "Sample input data..."]
