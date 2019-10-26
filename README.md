WIP - joint channel opening of multiple c-lightning peers via a signle transaction

### Status

Currently, 2 parties are able to open channels, however this is no where near 
production ready and is in the prototype stage.  

There are 2 plugins, a client and coordinator.  Clients request
to join with the coordinator, when the threshold is reached, a transaction is created
and each client signs and sends back to coordinator. 
Once all clients have signed and advised the coordinator that they have completed
the channel opening(fundchannel_complete c-lightning command) with txid and output index, 
the transaction is broadcast

### RPC command

`joinmulti_start HOST [{"id":"032e7...", "satoshi":n, "announce": true}...]`

called from the client to the HOST or coordinator will queue the channel opening
with the coordinator for channel with peer with `id` for n `satoshi`.

[demo video](https://youtu.be/QVYWQ3c8yM0)