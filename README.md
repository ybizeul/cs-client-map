## Description

`cs-client-map` is a go binary you can use to extract from NetApp CloudSecure
Activity API a list of all the clients using the volumes from ONTAP systems.

You can specify a date range and/or a path depth to match (usefull to map
based on qtrees instead of volumes)

```
./cs-client-map -h
Usage of ./cs-client-map:
  -e string
        CloudSecure enpoint for the instance to use, i.e. 'psxxx.cs01.cloudinsights.netapp.com'. Can be set in CS_ENDPOINT environement variable too
  -f int
        From time. Unix ms timestamp which defaults to yesterday at 00:00
  -k string
        API Key used to authenticate with CloudSecure service. Can be set in CS_API_KEY environement variable too
  -p int
        Path depth to output (default 1)
  -t int
        To time. Unix ms timestamp which defaults to today at 00:00
  -v    Print version and exits
```

Example output :
```
./cs-client-map
Analyzing 10777 records between
  18 Feb 23 00:00 CET and
  19 Feb 23 00:00 CET
Done 100%
192.168.0.5     /Marketing
10.192.48.70    /ENG_CIFS_volume
172.20.1.170    /Marketing
```

You can redirect output to a file :
```
./cs-client-map > out.txt
Analyzing 10777 records between
  18 Feb 23 00:00 CET and
  19 Feb 23 00:00 CET
Done 100%
```

### Using Proxies

You can set environment variables `HTTP_PROXY` and `HTTPS_PROXY` before running `cs-client-map`