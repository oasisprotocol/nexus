## Summary
(Locust)[https://locust.io/] is a load testing tool that we use to simulate users of the (Explorer)[https://explorer.oasis.io/]. See the docstring in `locustfile.py` for details.

`locust.conf` holds the locust configuration parameters. All of them can be overridden using command-line arguments. Run `locust -h` for more info.

In addition to the built-in cli options, we have two custom options:
```
--paratime <paratime>      Configures which paratime to query. Currently 
                           only supports 'emerald' and 'sapphire'. 
                           Consensus Explorer pages are not yet defined 
                           and are thus not supported yet. Defaults to 
                           'emerald'.

--sample-pool-size         Configures the size of the sample pool of 
                           recent addresses/txs to query during the load 
                           test. Defaults to 100.
```

## To run locally
```
pip3 install locust
locust
```
Or
```
locust --host https://nexus.stg.oasis.io --paratime emerald
```

## To view the Web UI
Go to http://localhost:8089
