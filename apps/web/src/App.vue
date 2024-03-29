<script setup lang="ts">
import Button from 'primevue/button';
import Card from 'primevue/card';
import InputNumber from 'primevue/inputnumber';
import InputText from 'primevue/inputtext';
import Toast from 'primevue/toast';
import { useToast } from "primevue/usetoast";
import ConfirmDialog from 'primevue/confirmdialog';
import { useConfirm } from "primevue/useconfirm";
import ProgressSpinner from 'primevue/progressspinner';
import Tag from 'primevue/tag';
import { ref, onMounted, onUnmounted } from 'vue';
import * as ethers from "ethers";

const BridgeContract = "0x1c3ba4cfb7a9c9c1617c28d5c91160426859895f" //testnet bridge contract
const GasScriptHash = "0xd2a4cff31913016155e38e474a2c06d08be276cf";
const NeoEVMChainID = "0x2d5311";
const NeoEVMRPCs = ["https://evm.ngd.network:32332"];

const AddressMatcher = /^0x[a-fA-F0-9]{40}$/g

const EthNetwork = new Map();
EthNetwork.set("0x1", "Mainnet");
EthNetwork.set("0x3", "Ropsten");
EthNetwork.set("0x4", "Rinkeby");
EthNetwork.set("0x5", "Goerli");
EthNetwork.set("0x2a", "Kovan");
EthNetwork.set(NeoEVMChainID, "NeoEVM");

enum Network {
  N3PrivateNet = 0,
  N2MainNet,
  N2TestNet,
  N3MainNet,
  N3TestNet = 6,
}
enum State {
  None,
  Pending,
  Confirmed,
  Relaying,
  Minted,
}
const toast = useToast();
const confirm = useConfirm();
let amount = ref(0);
let eaddress = ref("");
let naddress = ref("");
let network = ref(7);
let balance = ref("");
let dtxid = ref("");
let mtxid = ref("");
let dtxlink = ref("");
let mtxlink = ref("");
let neoline: any;
let metamask: any;
let maddress = ref("");
let mbalance = ref("");
let mchain = ref("");
let state = ref(State.None);

onMounted(() => {
  window.addEventListener('NEOLine.N3.EVENT.READY', async () => {
    neoline = new (window as any).NEOLineN3.Init();
    window.addEventListener('NEOLine.NEO.EVENT.NETWORK_CHANGED', handleNLNetworkChanged);
    window.addEventListener('NEOLine.NEO.EVENT.ACCOUNT_CHANGED', handleNLAccountChanged);
    window.addEventListener('NEOLine.NEO.EVENT.TRANSACTION_CONFIRMED', handleNLTransactionConfirmed);
  });
  if (typeof (window as any).ethereum !== 'undefined') {
    metamask = (window as any).ethereum;
    metamask.on('accountsChanged', handleMMAccountChanged);

    metamask.on('chainChanged', handleMMNetworkChanged);
  }
})

onUnmounted(() => {
  if (metamask != null) {
    metamask.removeListener('accountsChanged', handleMMAccountChanged);
    metamask.removeListener('chainChanged', handleMMNetworkChanged);
  }
  if (neoline != null) {
    window.removeEventListener('NEOLine.NEO.EVENT.NETWORK_CHANGED', handleNLNetworkChanged);
    window.removeEventListener('NEOLine.NEO.EVENT.ACCOUNT_CHANGED', handleNLAccountChanged);
    window.removeEventListener('NEOLine.NEO.EVENT.TRANSACTION_CONFIRMED', handleNLTransactionConfirmed);
  }
})

async function handleNLAccountChanged(result: any) {
  let addr = result.detail.address;
  naddress.value = addr;
  showInfo("account changed " + addr);
  await loadAccountInfo();
}

async function handleNLNetworkChanged(result: any) {
  network.value = result.detail.chainId as Network;
  showInfo(`switch to ${Network[network.value]}`);
  await loadAccountInfo();
}

async function handleNLTransactionConfirmed(result: any) {
  let tid = result.detail.txid;
  if (state.value == State.Pending && tid == dtxid.value) {
    await onTransactionConfirmed(tid);
  }
}

function handleMMAccountChanged(accounts: any) {
  maddress.value = accounts[0];
  eaddress.value = maddress.value;
  showInfo("evm layer account changed " + maddress.value);
}

function handleMMNetworkChanged(chainId: any) {
  mchain.value = chainId
  showInfo("evm layer network changed " + EthNetwork.get(mchain.value));
}

function resetState() {
  dtxid.value = "";
  mtxid.value = "";
  state.value = State.None;
}

async function deposit() {
  if (neoline == null) {
    showError("neoline not ready!");
    return;
  }
  if (naddress.value === "") {
    showError("wallet unconnected!");
    return;
  }
  if (network.value !== Network.N3TestNet) {
    showError("only N3Testnet supported");
    return;
  }
  resetState();
  const Method = "transfer";
  const fromScriptHash = (await neoline.AddressToScriptHash({ address: naddress.value })).scriptHash;
  let from = { type: "Hash160", value: fromScriptHash };
  let to = { type: "Hash160", value: BridgeContract };
  if (amount.value < 1) {
    showError("deposit amount shouldn't be less than 1GAS");
    return;
  }
  let value = { type: "Integer", value: BigInt(amount.value * 100000000).toString() };
  let eaddr = eaddress.value.match(AddressMatcher);
  if (eaddr == null || eaddr[0] === "0x0000000000000000000000000000000000000000") {
    showError("invalid address");
    return;
  }
  let address = { type: "Hash160", value: eaddr[0] };
  let signer = { account: fromScriptHash, scopes: 1 }; // callbyentry
  let invokeObj = {
    scriptHash: GasScriptHash,
    operation: Method,
    args: [from, to, value, address],
    signers: [signer],
  };
  askConfirm(invokeObj);
}

async function loadAccountInfo() {
  let results = await neoline.getBalance({
    params: [
      {
        address: naddress.value,
        contracts: [GasScriptHash],
      }
    ]
  });
  balance.value = results[naddress.value][0].amount;
}

async function loadMAccountInfo() {
  mbalance.value = ethers.formatEther(await metamask.request({ method: "eth_getBalance", params: [maddress.value] }));
}

async function connectNeoLine() {
  if (neoline == null) {
    showError("neoline not ready!");
    return;
  }
  if (naddress.value !== "") {
    showInfo("wallet already connected!");
    return;
  }
  await neoline.switchWalletNetwork({
    chainId: Network.N3TestNet,
  });
  network.value = Network.N3TestNet;
  let account = await neoline.getAccount();
  naddress.value = account.address;
  await loadAccountInfo();
}

async function switchMetaMaskNetwork() {
  try {
    await metamask.request({
      method: 'wallet_switchEthereumChain',
      params: [{ chainId: NeoEVMChainID }],
    });
    mchain.value = NeoEVMChainID;
  } catch (switchError: any) {
    // This error code indicates that the chain has not been added to MetaMask.
    if (switchError.code === 4902) {
      try {
        await metamask.request({
          method: 'wallet_addEthereumChain',
          params: [
            {
              chainId: NeoEVMChainID,
              chainName: 'NeoEVM',
              rpcUrls: NeoEVMRPCs,
              nativeCurrency: {
                name: "gas",
                symbol: "GAS",
                decimals: 18,
              },
            },
          ],
        });
        mchain.value = NeoEVMChainID;
        return;
      } catch (addError) {
        showError("can't add NeoEVM into MetaMask!");
        return;
      }
    }
    showError(switchError.message);
  }
}

async function connectMetaMask() {
  if (metamask == null) {
    showError("metamask not ready!");
    return;
  }
  await switchMetaMaskNetwork();
  let accounts = await metamask.request({ method: 'eth_requestAccounts' });
  maddress.value = accounts[0];
  eaddress.value = accounts[0];
  await loadMAccountInfo();
}

async function getDepositedId(txid: string) {
  let applog = await neoline.getApplicationLog({ txid: txid });
  let id = applog.executions[0].notifications[1].state.value[0].value;
  return Number(id);
}

function scriptToGetMinted(id: Number) {
  const iface = new ethers.Interface('[{"type":"function","name":"getMinted","inputs":[{"name":"a","type":"int64"}],"outputs":[{"name":"result","type":"bytes"}],"stateMutability":"nonpayable view"}]');
  return iface.encodeFunctionData("getMinted(int64)", [id]);
}

function getMinted(id: Number) {
  const From = "0xe9dea4d990d12ce5e3c7f19f9fcc2fe28c95b0c2"; // any address with GAS
  const Bridge = "0x00000000000000000000000000000000000000E5";
  const BlockscoutExplorerPrefix = 'http://evm.ngd.network/tx/';
  const EmptyHash = "0x" + "00".repeat(32);
  let timer = setInterval(async () => {
    try {
      let data = scriptToGetMinted(id);
      let txid = await metamask.request({
        method: "eth_call", params: [{
          from: From,
          to: Bridge,
          data: data,
        }]
      });
      if (txid != null && txid != EmptyHash) {
        mtxid.value = txid;
        clearInterval(timer);
        state.value = State.Minted;
        mtxlink.value = BlockscoutExplorerPrefix + txid;
        await loadMAccountInfo();
      }
    } catch (e: any) {
      console.log("can't get minted: " + e.message);
      clearInterval(timer);
    }
  }, 500);
}

async function onTransactionConfirmed(tid: string) {
  state.value = State.Relaying;
  mtxid.value = "relaying";
  await loadAccountInfo();
  let id = await getDepositedId(tid);
  getMinted(id);
}

function showError(msg: string) {
  toast.add({ severity: 'error', summary: 'Error', detail: msg, life: 3000, group: "info" });
}

function showInfo(msg: string) {
  toast.add({ severity: 'info', summary: 'Info', detail: msg, life: 3000, group: "info" });
}

function askConfirm(invokeObj: any) {
  confirm.require({
    message: `Are you sure you want to deposit ${amount.value}GAS to ${eaddress.value}?`,
    header: 'Confirmation',
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      await onConfirm(invokeObj);
    },
    reject: () => {
      onReject();
    }
  });
}

async function onConfirm(invokeObj: any) {
  toast.removeGroup("conf");
  try {
    dtxid.value = (await neoline.invoke(invokeObj)).txid;
    dtxlink.value = "https://testmagnet.explorer.onegate.space/transactionInfo/" + dtxid.value;
    state.value = State.Pending;
    showInfo(`transaction sent!`);
  } catch (e: any) {
    console.log(e);
    showError(e.description);
  }
}

function onReject() {
  toast.removeGroup("conf");
}
</script>

<template>
  <header>
  </header>

  <main>
    <div class="flex flex-column" style="width: 100%;">

      <Toast position="top-center" group="info" />
      <ConfirmDialog></ConfirmDialog>

      <div class="p-inputgroup flex-1 justify-content-end">
        <span class="p-inputgroup-addon">{{ balance }}GAS</span>
        <span class="p-inputgroup-addon">{{ naddress }}</span>
        <span class="p-inputgroup-addon">{{ Network[network] }}</span>
        <Button label="ConnectNeoLine" @click="connectNeoLine" />
      </div>

      <div class="p-inputgroup flex-1 justify-content-end" style="margin-top: 3rem;">
        <span class="p-inputgroup-addon">{{ mbalance }}GAS</span>
        <span class="p-inputgroup-addon">{{ maddress }}</span>
        <span class="p-inputgroup-addon">{{ EthNetwork.get(mchain) }}</span>
        <Button label="ConnectMetaMask" @click="connectMetaMask" />
      </div>

      <div class="flex align-items-center justify-content-center">
        <Card style="width: 40em; margin-top: 3rem;">
          <template #header>
            <img alt="user header" src="./assets/usercard.png" />
          </template>
          <template #title> NeoEVM Bridge </template>
          <template #subtitle> Deposit to NeoEVM layer </template>
          <template #content>
            <div class="p-inputgroup flex-1" style="margin-top: 1rem;">
              <span class="p-inputgroup-addon">NeoEVM address</span>
              <InputText type="text" v-model="eaddress" placeholder="0x0000000000000000000000000000000000000000" />
            </div>
            <div class="flex justify-content-end">
              <small id="username-help" style="margin-top: 1rem;">Total:{{ balance }}GAS</small>
            </div>
            <div class="p-inputgroup flex-1">
              <span class=" p-inputgroup-addon">amount</span>
              <InputNumber v-model="amount" inputId="minmaxfraction" :maxFractionDigits="8" />
            </div>
            <div class="flex flex-row align-items-center justify-content-between" v-show="state != State.None"
              style="width: 100%;margin-top: 1rem;">
              <ProgressSpinner aria-label="pending" strokeWidth="6" v-show="state == State.Pending"
                style="width: 30px; height: 30px;margin-left: -2px;margin-right: -1pt;" />
              <Tag v-show="state >= State.Confirmed" severity="success" value="deposited"></Tag>
              <a v-bind:href="dtxlink" style="font-size: smaller;color: gray;">{{
                dtxid }}</a>
            </div>
            <div class="flex flex-row align-items-center justify-content-between" v-show="state != State.None"
              style="width: 100%;margin-top: 1rem;">
              <ProgressSpinner aria-label="relaying" strokeWidth="6" v-show="state == State.Relaying"
                style="width: 30px; height: 30px;margin-left: -2px;margin-right: -1pt;" />
              <Tag v-show="state >= State.Minted" severity="success" value="minted"></Tag>
              <a v-bind:href="mtxlink" style="font-size: smaller;color: gray;">{{ mtxid }}</a>
            </div>
          </template>
          <template #footer>
            <Button label="Deposit"
              v-bind:disabled="(state >= State.Pending && state < State.Minted) || network != Network.N3TestNet"
              @click="deposit" />
          </template>
        </Card>
      </div>
    </div>
  </main>
</template>

<style scoped>
</style>
