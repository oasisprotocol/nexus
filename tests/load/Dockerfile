FROM python:3

WORKDIR /home/locust

RUN apt-get update && apt-get install -y vim git
RUN pip3 install locust

COPY ./locust* ./

CMD [ "locust" ]
