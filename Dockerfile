FROM scratch

COPY ./init ./init

ENTRYPOINT ["./init"]

CMD [ "--help" ]