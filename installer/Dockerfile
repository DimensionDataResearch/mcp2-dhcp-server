FROM ubuntu
RUN apt-get update
RUN apt-get install -y tftpd-hpa
RUN apt-get install -y apache2
RUN apt-get install -y ansible
#RUN apt-get install -y wget

#RUN mkdir /var/www/html/rancheros
#RUN mkdir /var/www/html/rancheros/1.0.4 
#RUN mkdir /var/www/html/rancheros/1.1

ADD ./dockerfiles/undionly.kpxe /var/lib/tftpboot
ADD ./dockerfiles/mcp2-dhcp-server /usr/local/bin
ADD ./dockerfiles/cloud-config-server /usr/local/bin

ADD ./dockerfiles/rancheros /var/www/html/rancheros

ADD ./templates /var/ansible/templates



