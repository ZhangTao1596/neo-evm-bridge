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
import { ref, onMounted } from 'vue';

const BridgeContract = "0xe96abc05434f03259a9af5d5454ab2db5739e8d0" //testnet bridge contract
const GasScriptHash = "0xd2a4cff31913016155e38e474a2c06d08be276cf";

const AddressMatcher = /^0x[a-fA-F0-9]{40}$/g
enum Network {
  N3PrivateNet = 0,
  N2MainNet,
  N2TestNet,
  N3MainNet,
  N3TestNet = 6,
}
const toast = useToast();
const confirm = useConfirm();
let amount = ref(0);
let eaddress = ref("0x0000000000000000000000000000000000000000");
let user = ref("");
let neoline: any;
let network = ref(7);
let balance = ref("");
let invokeObj = {};

onMounted(() => {
  window.addEventListener('NEOLine.N3.EVENT.READY', async () => {
    console.log(window.NEOLineN3);
    neoline = new window.NEOLineN3.Init();
  });
  window.addEventListener('NEOLine.NEO.EVENT.NETWORK_CHANGED', (result) => {
    network.value = result.detail.chainId as Network;
    showInfo(`switch to ${Network[network.value]}`);
  });
  window.addEventListener('NEOLine.NEO.EVENT.CONNECTED', (result) => {
    console.log('connected account:', result.detail);
  });
  window.addEventListener('NEOLine.NEO.EVENT.ACCOUNT_CHANGED', (result) => {
    console.log('account changed:', result.detail);
  });
  window.addEventListener('NEOLine.NEO.EVENT.BLOCK_HEIGHT_CHANGED', (result) => {
    console.log('block height:', result.detail);
  });
  window.addEventListener('NEOLine.NEO.EVENT.TRANSACTION_CONFIRMED', (result) => {
    console.log('Transaction confirmation detail:', result.detail);
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

  let address = { type: "Hash160", value: eaddress.value };
  let signer = { account: fromScriptHash, scopes: 1 };
  invokeObj = {
    scriptHash: GasScriptHash,
    operation: Method,
    args: [from, to, value, address],
    signers: [signer],
  };
  askConfirm();
}

async function switchAccount() {
  if (neoline == null) {
    showError("neoline not ready!");
    return;
  }
  let account = await neoline.switchWalletAccount();
  user.value = account.address;
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
  console.log("get balance");
  let results = await neoline.getBalance({
    params: [
      {
        address: account.address,
        contracts: [GasScriptHash],
      }
    ]
  });
  console.log(results);
  balance.value = results[account.address][0].amount;
}

function showError(msg: string) {
  toast.add({ severity: 'error', summary: 'Error', detail: msg, life: 3000, group: "info" });
}

function showInfo(msg: string) {
  toast.add({ severity: 'info', summary: 'Info', detail: msg, life: 3000, group: "info" });
}

function askConfirm() {
  confirm.require({
    message: `Are you sure you want to deposit ${amount.value} to ${eaddress.value}?`,
    header: 'Confirmation',
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      await onConfirm();
    },
    reject: () => {
      onReject();
    }
  });
}

async function onConfirm() {
  console.log("yes");
  toast.removeGroup("conf");
  try {
    let txid = (await neoline.invoke(invokeObj)).txid;
    showInfo(`deposit success! ${txid}`);
  } catch (e: any) {
    console.log(e);
    showError(e.description);
  }
}

function onReject() {
  console.log("no");
  toast.removeGroup("conf");
  invokeObj = {};
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
        <span class="p-inputgroup-addon">{{ balance }}</span>
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
            <div class="p-inputgroup flex-1" style="margin-top: 1rem;">
              <span class=" p-inputgroup-addon">amount</span>
              <InputNumber v-model="amount" inputId="minmaxfraction" :maxFractionDigits="8" />
            </div>
          </template>
          <template #footer>
            <Button label="Deposit" @click="deposit" style="margin-top: 1rem;" />
          </template>
        </Card>
      </div>
    </div>
  </main>
</template>

<style scoped>
</style>
