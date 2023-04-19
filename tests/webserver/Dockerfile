FROM nginx

ARG BUILD_DATE

ENV DEBIAN_FRONTEND=noninteractive

COPY ./nginx.conf /etc/nginx/nginx.conf

COPY ./*.html /usr/share/nginx/html/
