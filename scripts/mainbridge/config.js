module.exports = {
    defaultNetwork: "n3testnet",
    networks: {
        n3testnet: {
            url: "http://seed1t5.neo.org:20332",
            wif: "Kzj1LbTtmfbyJjn4cZhD6U4pdq74iHcmKmGRRBiLQoQzPBRWLEKz",
        },
        n3mainnet: {
            url: "http://seed1.neo.org:10332",
            wif: "",
        },
        private: {
            url: "http://localhost:10332",
            wif: "",
        }
    }
}
