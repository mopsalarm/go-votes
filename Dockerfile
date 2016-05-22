FROM scratch
COPY go-votes /go-votes
EXPOSE 8080
ENTRYPOINT ["/go-votes"]
