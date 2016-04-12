FROM alpine:3.2
ADD federation-srv /federation-srv
ENTRYPOINT [ "/federation-srv" ]
