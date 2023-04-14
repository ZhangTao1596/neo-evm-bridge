<script setup lang="ts">
import Avatar from 'primevue/avatar';
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
import { ref, onMounted } from 'vue';

const BridgeContract = "0x1c3ba4cfb7a9c9c1617c28d5c91160426859895f" //testnet bridge contract
const GasScriptHash = "0xd2a4cff31913016155e38e474a2c06d08be276cf";

const AddressMatcher = /^0x[a-fA-F0-9]{40}$/g
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
}
const toast = useToast();
const confirm = useConfirm();
let amount = ref(0);
let eaddress = ref("");
let user = ref("");
let network = ref(7);
let balance = ref("");
let txid = ref("");
let txlink = ref("")
let neoline: any;
let state = ref(State.None);

onMounted(() => {
  window.addEventListener('NEOLine.N3.EVENT.READY', async () => {
    console.log(window.NEOLineN3);
    neoline = new window.NEOLineN3.Init();
  });
  window.addEventListener('NEOLine.NEO.EVENT.NETWORK_CHANGED', async (result) => {
    network.value = result.detail.chainId as Network;
    showInfo(`switch to ${Network[network.value]}`);
    await loadAccountInfo();
  });
  window.addEventListener('NEOLine.NEO.EVENT.ACCOUNT_CHANGED', async (result) => {
    let addr = result.detail.address;
    user.value = addr;
    showInfo("account changed " + addr);
    await loadAccountInfo();
  });
  window.addEventListener('NEOLine.NEO.EVENT.TRANSACTION_CONFIRMED', async (result) => {
    let tid = result.detail.txid;
    if (state.value == State.Pending && tid == txid.value) {
      await onTransactionConfirmed();
    }
  });
})

async function deposit() {
  if (neoline == null) {
    showError("neoline not ready!");
    return;
  }
  if (user.value === "") {
    showError("wallet unconnected!");
    return;
  }
  const Method = "transfer";
  const fromScriptHash = (await neoline.AddressToScriptHash({ address: user.value })).scriptHash;
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

async function switchAccount() {
  if (neoline == null) {
    showError("neoline not ready!");
    return;
  }
  let account = await neoline.switchWalletAccount();
  user.value = account.address;
}

async function loadAccountInfo() {
  let results = await neoline.getBalance({
    params: [
      {
        address: user.value,
        contracts: [GasScriptHash],
      }
    ]
  });
  balance.value = results[user.value][0].amount;
}

async function connectNeoLine() {
  if (neoline == null) {
    showError("neoline not ready!");
    return;
  }
  if (user.value !== "") {
    showInfo("wallet already connected!");
    return;
  }
  await neoline.switchWalletNetwork({
    chainId: Network.N3TestNet,
  });
  network.value = Network.N3TestNet;
  let account = await neoline.getAccount();
  user.value = account.address;
  await loadAccountInfo();
}

async function onTransactionConfirmed() {
  state.value = State.Confirmed;
  await loadAccountInfo();
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
  console.log("yes");
  toast.removeGroup("conf");
  try {
    txid.value = (await neoline.invoke(invokeObj)).txid;
    txlink.value = "https://testmagnet.explorer.onegate.space/transactionInfo/" + txid.value;
    state.value = State.Pending;
    showInfo(`transaction sent!`);
  } catch (e: any) {
    console.log(e);
    showError(e.description);
  }
}

function onReject() {
  console.log("no");
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
        <span class="p-inputgroup-addon">{{ user }}</span>
        <span class="p-inputgroup-addon">{{ Network[network] }}</span>
        <Avatar icon="pi pi-user" class="mr-2" size="xlarge" @click="switchAccount" />
        <Button label="ConnectWallet" @click="connectNeoLine" />
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
            <div class="flex flex-row align-items-center justify-content-start" v-show="state != State.None"
              style="width: 100%;margin-top: 1rem;">
              <ProgressSpinner aria-label="pending" strokeWidth="4" v-show="state == State.Pending"
                style="width: 25px; height: 25px;margin-left: -2px;margin-right: -1pt;" />
              <Tag v-show="state == State.Confirmed" severity="success" value="Success"></Tag>
              <a v-bind:href="txlink" style="font-size: smaller;color: gray;">{{ txid }}</a>
            </div>
          </template>
          <template #footer>
            <Button label="Deposit" v-bind:disabled="state == State.Pending" @click="deposit" />
          </template>
        </Card>
      </div>
    </div>
  </main>
</template>

<style scoped>
</style>
