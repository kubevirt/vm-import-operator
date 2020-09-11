FROM centos:8

RUN mkdir /disks && \
    yum -y update && \
    rm -rf /var/cache/yum && \
    yum install -y \
        qemu-guest-agent \
        qemu-img \
        qemu-kvm \
        virt-v2v \
        virtio-win && \
    yum clean all

ENV LIBGUESTFS_BACKEND=direct

COPY build/virtv2v/bin /usr/local/bin

ENTRYPOINT ["/usr/local/bin/entrypoint"]
