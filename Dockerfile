FROM alpine:3.2
ADD zergkv /zergkv
ENTRYPOINT [ "/zergkv" ]