FROM busybox

ADD bin/gateway /bin/

EXPOSE 80
CMD gateway -port 80 -host local.pnorental.com -hostip `route | grep default | tr -s ' ' | cut -d' ' -f2`
