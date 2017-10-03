FROM ubuntu:17.04
RUN apt-get update
RUN apt-get install -y tftpd-hpa
RUN apt-get install -y apache2
RUN apt-get install -y ansible
RUN  rm -rf /var/lib/apt/lists/*
#RUN apt-get install -y wget

#RUN mkdir /var/www/html/rancheros
#RUN mkdir /var/www/html/rancheros/1.0.4 
#RUN mkdir /var/www/html/rancheros/1.1

ADD ./dockerfiles/undionly.kpxe /var/lib/tftpboot
ADD ./dockerfiles/mcp2-dhcp-server /usr/local/bin
ADD ./dockerfiles/cloud-config-server /usr/local/bin

RUN mkdir /var/ansible
RUN mkdir /var/ansible/templates
RUN mkdir /var/ansible/group_vars



ADD ./dockerfiles/rancheros /var/www/html/rancheros

ADD ./ansible-localhost-docker/inventory /var/ansible
ADD ./ansible-localhost-docker/templates /var/ansible/templates
ADD ./ansible-localhost-docker/group_vars /var/ansible/group_vars
ADD ./ansible-localhost-docker/dockerup.yml /var/ansible


# Service Startups
ADD ./files/mcp2-dhcp-server.service /lib/systemd/system/mcp2-dhcp-server.service
RUN ln -s /lib/systemd/system/mcp2-dhcp-server.service /etc/systemd/system/multi-user.target.wants/mcp2-dhcp-server.service
RUN chmod 755 /lib/systemd/system/mcp2-dhcp-server.service
RUN chmod 755 /etc/systemd/system/multi-user.target.wants/mcp2-dhcp-server.service

ADD ./files/cloud-config-server.service /lib/systemd/system/cloud-config-server.service
RUN ln -s /lib/systemd/system/cloud-config-server.service /etc/systemd/system/multi-user.target.wants/cloud-config-server.service
RUN chmod 755 /lib/systemd/system/cloud-config-server.service
RUN chmod 755 /etc/systemd/system/multi-user.target.wants/cloud-config-server.service
ADD ./dockerfiles/dockerentry.sh /var/ansible
RUN chmod 755 /var/ansible/dockerentry.sh



EXPOSE 80
EXPOSE 69
EXPOSE 67
EXPOSE 12123


