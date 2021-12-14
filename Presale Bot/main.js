const ethers = require("ethers");
const config = require('./config.json')
const ethUtils = require('ethereumjs-util')
const approveToken = require('./approve.js')

const provider = new ethers.providers.WebSocketProvider(config["provider"]);
const wallet = new ethers.Wallet(config["privateKey"], provider)


async function storage(storageInt) {
    return await provider.getStorageAt(config["presaleAddress"], storageInt);
}

const storageAddress = {
  pinkSale : {
    startTime : 106,
    minBuy : 130,
    maxBuy : 131,
    hardCap : 133,
    tokenAddress : 105,
    startBlock: 0
  },
  dxSale : {
    startTime : 45,
    minBuy : 26,
    maxBuy : 27,
    hardCap : 44,
    tokenAddress : 0,
    startBlock: 0
  },
  unicrypt : {
    startTime : 0,
    minBuy : 0,
    maxBuy : 5,
    hardCap : 7,
    tokenAddress : 2,
    startBlock: 11
  }
}

const wait = async () => {
  async function data(data) {
    startTime = parseInt(await storage(data.startTime), 16)
    minBuy = ethers.BigNumber.from(parseInt((await storage(data.minBuy)), 16).toString())
    maxBuy = ethers.BigNumber.from((await storage(data.maxBuy)).toString())
    hardCap = ethers.BigNumber.from(parseInt((await storage(data.hardCap)), 16).toString())
    tokenAddress = "0x" + (await storage(data.tokenAddress)).toString().slice(26, 66)
    startBlock = parseInt(await storage(data.startBlock), 16)
  }
  let zero = ethers.BigNumber.from(0)
  let contributedAmount = zero;
  let nonce = (await provider.getTransactionCount('0x' + ethUtils.privateToAddress(Buffer.from(config["privateKey"].trim().toLowerCase(), 'hex')).toString('hex'))) // quite janky just make sure to use a fresh wallet
  
  if (config["action"] == "pinkSale") {
    await data(storageAddress.pinkSale)
  } else if (config["action"] == "dxSale") {
    await data(storageAddress.dxSale)
    hardCap = hardCap.mul(ethers.BigNumber.from(10).pow(19))
  } else if (config["action"] == "unicrypt") {
    await data(storageAddress.unicrypt)
    minBuy = zero
  }
  console.log(startTime + "\n" + minBuy + "\n" + maxBuy + "\n" + hardCap + "\n" + tokenAddress + "\n" + startBlock);
  approveToken(tokenAddress)
  if (config["action"] == "pinkSale" || config["action"] == "dxSale") {
    while (startTime - unix() > 4) { 
      process.stdout.write(startTime - unix() + " Seconds remaining\n");
      await new Promise(r => setTimeout(r, 500));
    }
  }
  if (config["action"] == "unicrypt") {
    while (await provider.getBlockNumber() < (startBlock - 2)) {
      console.log("Current Block:" + await provider.getBlockNumber());
      await new Promise(r => setTimeout(r, 1000));
    }
  }
  provider.on('pending', async (txHash) => { // Look through the mempool
      provider.getTransaction(txHash).then(async (tx) => { // Get the transaction from the hash
          if (tx && tx.to) {
            if (tx.to === config["presaleAddress"] || tx.data.slice(0, 10) == "0xf868e766" && tx.value.lte(maxBuy) && tx.value.gte(minBuy)) {
            contributedAmount = tx.value.add(contributedAmount)
            if (contributedAmount.gte(hardCap.mul(config["percentFill"]).div(100))) {
              let sendTx = { // Create the transaction
                value: ethers.BigNumber.from(config["amtBuy"]).mul(ethers.BigNumber.from(10).pow(18)).div(1000),
                nonce: nonce,
                gasPrice: (tx.gasPrice).add(1),
                gasLimit: (tx.gasLimit),
                chainId: parseInt(config["chainId"], 10),
                to: config["presaleAddress"],
                data: "0x"
              }
              let signedTx = await wallet.signTransaction(sendTx) // Sign the transaction
              let receipt = await provider.sendTransaction(signedTx) // Send the transaction to the network
              console.log("Buy receipt: " + receipt.hash + "\n");
              process.exit(1)
            }
          }
          }
      })
  })
}

function unix() {
    return parseInt(Math.floor(new Date().getTime() / 1000));
  }

wait()