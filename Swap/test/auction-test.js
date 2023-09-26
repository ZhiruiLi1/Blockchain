const { expect } = require("chai")
const { ethers } = require("hardhat")

//To create a S, H(S) pair, used https://emn178.github.io/online-tools/sha256.html
// 'secret' --> 0x736563726574
const preimage = "0x7365637265740000000000000000000000000000000000000000000000000000"
const wrongPreimage = "0x6365637265740000000000000000000000000000000000000000000000000000"
const hashLock = "0x497a39b618484855ebb5a2cabf6ee52ff092e7c17f8bfe79313529f9774f83a2"

let apricot;
let apricotSwap;
let alice;
let bob;
let startTime;

const setupContracts = async () => {
  [alice, bob] = await ethers.getSigners()
  const Apricot = await ethers.getContractFactory("Apricot")
  const TwoPartySwap = await ethers.getContractFactory("TwoPartySwap")
  apricotSwap = await TwoPartySwap.deploy()
  await apricotSwap.deployed()
  apricot = await Apricot.deploy("Apricot", "APR")
  await apricot.deployed()
  await apricot.transfer(bob.address, ethers.utils.parseEther("100"));
  await apricot.increaseAllowance(apricotSwap.address, ethers.utils.parseEther("100000"));
  await apricot.connect(bob).increaseAllowance(apricotSwap.address, ethers.utils.parseEther("100000"));
  startTime = (await ethers.provider.getBlock()).timestamp
}

const setupSwap = async (expectedAssetEscrow, expectedPremiumEscrow) => {
  await apricotSwap.setup(
    expectedAssetEscrow,
    expectedPremiumEscrow,
    alice.address,
    bob.address,
    apricot.address,
    hashLock,
    startTime,
    true,
    1000
  )
}

describe("Setup", function() {

  beforeEach(async function() {
    await setupContracts()
  })

  it("Should fail if address is not 0", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await expect(apricotSwap.setup(
      ethers.utils.parseEther("1000"),
      ethers.utils.parseEther("100"),
      alice.address,
      bob.address,
      apricot.address,
      hashLock,
      startTime,
      true,
      1000
    )).to.be.reverted
  })

  it("Should emit event with correct args if firstAssetEscrow == true", async function() {
    await expect(apricotSwap.setup(
      ethers.utils.parseEther("1000"),
      ethers.utils.parseEther("100"),
      alice.address,
      bob.address,
      apricot.address,
      hashLock,
      startTime,
      true,
      1000))
    .to.emit(apricotSwap, "SetUp").withArgs(
        alice.address,
        bob.address,
        ethers.utils.parseEther("100"),
        ethers.utils.parseEther("1000"),
        startTime,
        startTime + 2000,
        startTime + 3000,
        startTime + 6000
    )
  })

  it("Should emit event with correct args if firstAssetEscrow == false", async function() {
    await expect(apricotSwap.setup(
      ethers.utils.parseEther("1000"),
      ethers.utils.parseEther("100"),
      alice.address,
      bob.address,
      apricot.address,
      hashLock,
      startTime,
      false,
      1000))
    .to.emit(apricotSwap, "SetUp").withArgs(
        alice.address,
        bob.address,
        ethers.utils.parseEther("100"),
        ethers.utils.parseEther("1000"),
        startTime,
        startTime + 1000,
        startTime + 4000,
        startTime + 5000
    )
  })
})

describe("Escrow Premium", function() {

  beforeEach(async function() {
    await setupContracts()
  })

  it("Should fail if not in time", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await ethers.provider.send("evm_increaseTime", [2000])
    await expect(apricotSwap.connect(bob).escrowPremium(hashLock)).to.be.reverted
  }) 

  it("Should fail if not proper sender", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await expect(apricotSwap.escrowPremium(hashLock)).to.be.reverted
  })

  it("Should fail if premium already escrowed", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await apricotSwap.connect(bob).escrowPremium(hashLock)
    await expect(apricotSwap.connect(bob).escrowPremium(hashLock)).to.be.reverted
  })

  it("Should fail if not enough balance", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("1000"))
    await expect(apricotSwap.connect(bob).escrowPremium(hashLock)).to.be.reverted
  })

  it("Should emit event with correct args", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await expect(apricotSwap.connect(bob).escrowPremium(hashLock))
      .to.emit(apricotSwap, "PremiumEscrowed").withArgs(
        bob.address,
        ethers.utils.parseEther("100"),
        bob.address,
        apricotSwap.address,
        ethers.utils.parseEther("100"),
        ethers.utils.parseEther("0"),
      );
  })  

  it("Should reduce premiumEscrower's balance", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await expect(() => apricotSwap.connect(bob).escrowPremium(hashLock)).to.changeTokenBalances(
      apricot, 
      [bob, apricotSwap], 
      [ethers.utils.parseEther("-100"), ethers.utils.parseEther("100")],
    )
  })
})

describe("Escrow Asset", function() {

  beforeEach(async function() {
    await setupContracts()
  })

  const setupAssetEscrow = async () => {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await apricotSwap.connect(bob).escrowPremium(hashLock)
  }

  it("Should fail if not in time", async function() {
    await setupAssetEscrow()
    await ethers.provider.send("evm_increaseTime", [3000])
    await expect(apricotSwap.escrowAsset(hashLock)).to.be.reverted
  }) 
  
  it("Should fail if not proper sender", async function() {
    await setupAssetEscrow()
    await expect(apricotSwap.connect(bob).escrowAsset(hashLock)).to.be.reverted
  })

  it("Should fail if premium not escrowed before asset", async function() {  
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await expect(apricotSwap.escrowAsset(hashLock)).to.be.reverted
  })
  
  it("Should fail if asset not escrowed or escrowed more than once", async function() {
    await setupAssetEscrow()
    await apricotSwap.escrowAsset(hashLock)
    await expect(apricotSwap.escrowAsset(hashLock)).to.be.reverted
  })

  it("Should fail if not enough balance", async function() {
    await setupSwap(ethers.utils.parseEther("1000000"), ethers.utils.parseEther("100"))
    await apricotSwap.connect(bob).escrowPremium(hashLock)
    await expect(apricotSwap.escrowAsset(hashLock)).to.be.reverted
  })

  it("Should emit event with correct args", async function() {
    await setupAssetEscrow()
    await expect(apricotSwap.escrowAsset(hashLock))
      .to.emit(apricotSwap, "AssetEscrowed").withArgs(
        alice.address,
        ethers.utils.parseEther("1000"),
        alice.address,
        apricotSwap.address,
        ethers.utils.parseEther("100"),
        ethers.utils.parseEther("1000"),
      );
  })

  it("Should reduce asset escrower's balance", async function() {
    await setupAssetEscrow()
    await expect(() => apricotSwap.escrowAsset(hashLock)).to.changeTokenBalances(
      apricot, 
      [alice, apricotSwap], 
      [ethers.utils.parseEther("-1000"), ethers.utils.parseEther("1000")],
    )
  })
})

describe("Redeem Asset", function() {

  const setupAssetRedeem = async () => {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await apricotSwap.connect(bob).escrowPremium(hashLock)
    await apricotSwap.escrowAsset(hashLock)
  }

  beforeEach(async function() {
    await setupContracts()
  })

  it("Should fail if not in time", async function() {
    await setupAssetRedeem()
    await ethers.provider.send("evm_increaseTime", [6000])
    await expect(apricotSwap.connect(bob).redeemAsset(preimage, hashLock)).to.be.reverted
  }) 

  it("Should fail if not proper sender", async function() {
    await setupAssetRedeem()
    await expect(apricotSwap.redeemAsset(preimage, hashLock)).to.be.reverted
  })

  it("Should fail if preimage does not hash to hashLock", async function() {
    await setupAssetRedeem()
    await expect(apricotSwap.connect(bob).redeemAsset(wrongPreimage, hashLock)).to.be.reverted
  })

  it("Should fail if asset not escrowed", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await apricotSwap.connect(bob).escrowPremium(hashLock)
    await expect(apricotSwap.connect(bob).redeemAsset(preimage, hashLock)).to.be.reverted
  })

  it("Should properly adjust balances ", async function() {
    await setupAssetRedeem()
    await expect(() => apricotSwap.connect(bob).redeemAsset(preimage, hashLock))
      .to.changeTokenBalances(
        apricot, 
        [bob, apricotSwap], 
        [ethers.utils.parseEther("1000"), ethers.utils.parseEther("-1000")],
      )
  })

  it("Should emit event with correct args", async function() {
    await setupAssetRedeem()
    await expect(apricotSwap.connect(bob).redeemAsset(preimage, hashLock))
      .to.emit(apricotSwap, "AssetRedeemed").withArgs(
        bob.address,
        ethers.utils.parseEther("1000"),
        apricotSwap.address,
        bob.address,
        ethers.utils.parseEther("100"),
        ethers.utils.parseEther("0"),
      );
  })
})

describe ("Redeem Premium", function() {
  
    beforeEach(async function() {
      await setupContracts()
    })

    it("Should fail if premium not escrowed", async function() {
      await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
      await ethers.provider.send("evm_increaseTime", [6000])
      await expect(apricotSwap.redeemPremium(hashLock)).to.be.reverted
    })

    it("Should fail if expected asset amount is 0", async function() {
      await setupSwap(ethers.utils.parseEther("0"), ethers.utils.parseEther("100"))
      await apricotSwap.connect(bob).escrowPremium(hashLock)
      await ethers.provider.send("evm_increaseTime", [6000])
      await expect(apricotSwap.redeemPremium(hashLock)).to.be.reverted
    })

    it("Should fail if expected premium does not match current", async function() {
      await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
      await apricotSwap.connect(bob).escrowPremium(hashLock)
      await apricotSwap.escrowAsset(hashLock)
      await ethers.provider.send("evm_increaseTime", [6000])
      await apricotSwap.redeemPremium(hashLock)
      await expect(apricotSwap.redeemPremium(hashLock)).to.be.reverted
    })

    it("Should fail if it does not wait to timeout", async function() {
      await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
      await ethers.provider.send("evm_increaseTime", [3000])
      await expect(apricotSwap.redeemPremium(hashLock)).to.be.reverted
    })

    it("Should emit event with correct args", async function() {
      await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
      await apricotSwap.connect(bob).escrowPremium(hashLock)
      await apricotSwap.escrowAsset(hashLock)
      await ethers.provider.send("evm_increaseTime", [6000])
      await expect(apricotSwap.redeemPremium(hashLock))
        .to.emit(apricotSwap, "PremiumRedeemed").withArgs(
          alice.address,
          ethers.utils.parseEther("100"),
          apricotSwap.address,
          alice.address,
          ethers.utils.parseEther("0"),
          ethers.utils.parseEther("1000"),
        )
    })

    it("Should properly adjust balances", async function() {
      await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
      await apricotSwap.connect(bob).escrowPremium(hashLock)
      await apricotSwap.escrowAsset(hashLock)
      await ethers.provider.send("evm_increaseTime", [6000])
      await expect(() => apricotSwap.redeemPremium(hashLock))
      .to.changeTokenBalances(
        apricot, 
        [alice, apricotSwap], 
        [ethers.utils.parseEther("100"), ethers.utils.parseEther("-100")],
      )
    })
})

describe("Refund Asset", function() {

  const setupAssetRefund = async () => {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await apricotSwap.connect(bob).escrowPremium(hashLock)
    await apricotSwap.escrowAsset(hashLock)
  }

  beforeEach(async function() {
    await setupContracts()
  })

  it("Should fail if too early", async function() {
    await setupAssetRefund()
    await expect(apricotSwap.refundAsset(hashLock)).to.be.reverted
  })

  it("Should fail if premium not escrowed", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await ethers.provider.send("evm_increaseTime", [6000])
    await expect(apricotSwap.refundAsset(hashLock)).to.be.reverted
  })

  it("Should fail if asset not escrowed", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await apricotSwap.connect(bob).escrowPremium(hashLock)
    await ethers.provider.send("evm_increaseTime", [6000])
    await expect(apricotSwap.refundAsset(hashLock)).to.be.reverted
  })

  it("Should properly adjust balances", async function() {
    await setupAssetRefund()
    await ethers.provider.send("evm_increaseTime", [6000])
    await expect(() => apricotSwap.refundAsset(hashLock))
      .to.changeTokenBalances(
        apricot, 
        [alice, apricotSwap], 
        [ethers.utils.parseEther("1000"), ethers.utils.parseEther("-1000")],
      )
  })

  it("Should emit event with correct args", async function() {
    await setupAssetRefund()
    await ethers.provider.send("evm_increaseTime", [6000])
    await expect(apricotSwap.refundAsset(hashLock))
      .to.emit(apricotSwap, "AssetRefunded").withArgs(
        alice.address,
        ethers.utils.parseEther("1000"),
        apricotSwap.address,
        alice.address,
        ethers.utils.parseEther("100"),
        ethers.utils.parseEther("0"),
      )
  })
})

describe("Refund Premium", function() {

  const setupPremiumRefund = async () => {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await apricotSwap.connect(bob).escrowPremium(hashLock)
  }

  beforeEach(async function() {
    await setupContracts()
  })

  it("Should fail if bob premium not escrowed", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await ethers.provider.send("evm_increaseTime", [10000])
    await expect(apricotSwap.connect(bob).refundPremium(hashLock)).to.be.reverted
  })

  it("Should fail if alice has already escrowed asset", async function() {
    await setupPremiumRefund()
    await apricotSwap.escrowAsset(hashLock)
    await ethers.provider.send("evm_increaseTime", [10000])
    await expect(apricotSwap.connect(bob).refundPremium(hashLock)).to.be.reverted
  })

  it("Should fail if too early", async function() {
    await setupPremiumRefund()
    await expect(apricotSwap.connect(bob).refundPremium(hashLock)).to.be.reverted
  })

  it("Should fail if premium is not escrowed", async function() {
    await setupSwap(ethers.utils.parseEther("1000"), ethers.utils.parseEther("100"))
    await ethers.provider.send("evm_increaseTime", [10000])
    await expect(apricotSwap.connect(bob).refundPremium(hashLock)).to.be.reverted
  })

  it("Should properly adjust balances", async function() {
    await setupPremiumRefund()
    await ethers.provider.send("evm_increaseTime", [10000])
    await expect(() => apricotSwap.connect(bob).refundPremium(hashLock))
      .to.changeTokenBalances(
        apricot,
        [bob, apricotSwap],
        [ethers.utils.parseEther("100"), ethers.utils.parseEther("-100")],
      )
  })

  it("Should emit event with correct args", async function() {
    await setupPremiumRefund()
    await ethers.provider.send("evm_increaseTime", [10000])
    await expect(apricotSwap.connect(bob).refundPremium(hashLock))
      .to.emit(apricotSwap, "PremiumRefunded").withArgs(
        bob.address,
        ethers.utils.parseEther("100"),
        apricotSwap.address,
        bob.address,
        ethers.utils.parseEther("0"),
        ethers.utils.parseEther("0"),
      );
  })
})
