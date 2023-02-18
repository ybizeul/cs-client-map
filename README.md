## Description

`cs-client-map` is a go binary you can use to extract from NetApp CloudSecure
Activity API a list of all the clients using the volumes from ONTAP systems.

You can specify a date range and/or a path depth to match (usefull to map
based on qtrees instead of volumes)

```
cs-client-map -h
Usage of /var/folders/lb/634kc9l90njb7_lp5_nqgqxc001h8c/T/go-build660580836/b001/exe/main:
  -e string
        CloudSecure enpoint for the instance to use, i.e. 'psxxx.cs01.cloudinsights.netapp.com'. Can be set in CI_ENDPOINT environement variable too
  -f int
        From time. Unix timestamp which defaults to yesterday at 00:00
  -k string
        API Key used to authenticate with CloudSecure service. Can be set in CI_API_KEY environement variable too
  -p int
        Path depth to output, default is 1 (Volume) (default 1)
  -t int
        To time. Unix timestamp which defaults to today at 00:00
```

Example output :
```
./cs-client-map
10.192.48.70    /ENG_CIFS_volume
192.168.0.5     /Marketing
172.20.1.170    /Marketing
```