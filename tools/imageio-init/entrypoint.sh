#!/bin/bash
/imageio-init $1 $2
cat $2/tls.crt | awk 'split_after==1{n++;split_after=0} /-----END CERTIFICATE-----/ {split_after=1} {print > "cert" n ".pem"}'
mv cert1.pem $2/ca.pem
mv cert.pem $2
