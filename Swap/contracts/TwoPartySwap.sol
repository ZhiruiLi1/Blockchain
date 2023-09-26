// SPDX-License-Identifier: UNLICENSED

pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "hardhat/console.sol";

contract TwoPartySwap {

    /**
    The Swap struct keeps track of participants and swap details
     */
    struct Swap {
        // assetEscrower: who escrows the asset (Alice in diagram)
        address payable assetEscrower;
        // premiumEscrower: who escrows the premium (Bob in diagram)
        address payable premiumEscrower;
        // hashLock: the hash of a secret, which only the assetEscrower knows
        bytes32 hashLock;
        // assetAddress: the ERC20 Token's address, which will be used to access accounts
        address assetAddress;
    }

    /**
    The Asset struct keeps track of the escrowed Asset
     */
    struct Asset {
        // expected: the agreed-upon amount to be escrowed
        uint expected;
        // current: the current amount of the asset that is escrowed in the swap.
        uint current;
        // deadline: the time before which the person escrowing their asset must do so
        uint deadline;
        // timeout: the maximum time the protocol can take, which assumes everything
        // goes to plan.
        uint timeout;
    }

    /**
    The Premium struct keeps track of the escrowed premium.
     */
    struct Premium {
        // expected: the agreed-upon amount to be escrowed as a premium
        uint expected;
        // current: the current amount of the premium that is escrowed in the swap
        uint current;
        // deadline: the time before which the person escrowing their premium must do so
        uint deadline;
    }

    /**
    Mappings that store our swap details. This contract stores multiple swaps; you can access
    information about a specific swap by using its hashLock as the key to the appropriate mapping.
     */
    mapping(bytes32 => Swap) public swaps;
    mapping(bytes32 => Asset) public assets;
    mapping(bytes32 => Premium) public premiums;

    /**
    SetUp: this event should emit when a swap is successfully setup.
     */
    event SetUp(
        address payable assetEscrower,
        address payable premiumEscrower,
        uint expectedPremium,
        uint expectedAsset,
        uint startTime,
        uint premiumDeadline,
        uint assetDeadline,
        uint assetTimeout
    );

    /**
    PremiumEscrowed: this event should emit when the premiumEscrower successfully escrows the premium
     */
    event PremiumEscrowed (
        address messageSender,
        uint amount,
        address transferFrom,
        address transferTo,
        uint currentPremium,
        uint currentAsset
    );

    /**
    AssetEscrowed: this event should emit  when the assetEscrower successfully escrows the asset
     */
    event AssetEscrowed (
        address messageSender,
        uint amount,
        address transferFrom,
        address transferTo,
        uint currentPremium,
        uint currentAsset
    );

    /**
    AssetRedeemed: this event should emit when the assetEscrower successfully escrows the asset
     */
    event AssetRedeemed(
        address messageSender,
        uint amount,
        address transferFrom,
        address transferTo,
        uint currentPremium,
        uint currentAsset
    );

    /**
    PremiumRefunded: this event should emit when the premiumEscrower successfully gets their premium refunded
     */
    event PremiumRefunded(
        address messageSender,
        uint amount,
        address transferFrom,
        address transferTo,
        uint currentPremium,
        uint currentAsset
    );

    /**
    PremiumRedeemed: this event should emit when the counterparty breaks the protocol
    and the assetEscrower redeems the  premium for breaking the protocol 
     */
    event PremiumRedeemed(
        address messageSender,
        uint amount,
        address transferFrom,
        address transferTo,
        uint currentPremium,
        uint currentAsset
    );

    /**
    AssetRefunded: this event should emit when the counterparty breaks the protocol 
    and the assetEscrower succesffully gets their asset refunded
     */
    event AssetRefunded(
        address messageSender,
        uint amount,
        address transferFrom,
        address transferTo,
        uint currentPremium,
        uint currentAsset
    );

    /**
    TODO: using modifiers for your require statements is best practice,
    but we do not require you to do so
    */ 
    modifier canSetup(bytes32 hashLock) {
        require(swaps[hashLock].assetEscrower == address(0), "error, swap already existed!");
        _;
    }

    modifier canEscrowPremium(bytes32 hashLock) {
        require(block.timestamp <= premiums[hashLock].deadline, "error, deadline has passed!");
        require(swaps[hashLock].premiumEscrower == msg.sender, "error, you are not the premium escrower!");
        require(premiums[hashLock].current < premiums[hashLock].expected, "error, current premium has already completed!");
        _;
    }
    modifier canEscrowAsset(bytes32 hashLock) {
        require(assets[hashLock].current < assets[hashLock].expected, "error, assets already completed!");
        require(swaps[hashLock].assetEscrower == msg.sender, "error, you are not the asset escrower!");
        require(block.timestamp <= assets[hashLock].deadline, "error, deadline has passed!");
        require(premiums[hashLock].current >= premiums[hashLock].expected, "error, you haven't completed the premium!");
        _;
    }

    modifier canRedeemAsset(bytes32 preimage, bytes32 hashLock) {
        require(assets[hashLock].current == assets[hashLock].expected, "error, assets not completed!");
        require(block.timestamp <= assets[hashLock].deadline, "error, deadline has passed!");
        require(swaps[hashLock].premiumEscrower == msg.sender, "error, you are not the premium escrpwer!");
        require(sha256(abi.encode(preimage)) == hashLock, "error, preimage not matched!");
        // sha256(abi.encode(preimage)): This part of the code computes the SHA-256 hash of the preimage variable. 
        // First, abi.encode(preimage) is used to encode the preimage into a bytes representation according to the Ethereum ABI 
        // encoding rules. Then, sha256() is applied to compute the hash of the encoded data.

        _;
    }

    modifier canRefundAsset(bytes32 hashLock) {
        require(premiums[hashLock].current == premiums[hashLock].expected, "error, the premium is not correct!");
        require(assets[hashLock].current == assets[hashLock].expected, "error, the asset is not correct!");
        require(block.timestamp > assets[hashLock].deadline, "error, deadline has not passed!");
        _;
    }

    modifier canRefundPremium(bytes32 hashLock) {
        require(assets[hashLock].current < assets[hashLock].expected, "error, asserts are already completed!");
        require(block.timestamp > assets[hashLock].timeout, "error, timeout has not passed!");
        require(premiums[hashLock].current == premiums[hashLock].expected, "error, the premium is not correct!");
        _;
    }

    modifier canRedeemPremium(bytes32 hashLock) {
        require(swaps[hashLock].assetEscrower == msg.sender, "error, your are not the asset escrower!");
        require(premiums[hashLock].current == premiums[hashLock].expected, "error, premium is not completed!");
        require(assets[hashLock].expected > 0, "error, the asset amount should greater than 0!");
        require(block.timestamp > premiums[hashLock].deadline, "error, the deadline has not passed!");
        _;
        // In Solidity, block is a global variable that provides information about the current block being mined on the blockchain.
    }
   
    /**
    setup is called to initialize an instance of a swap in this contract. 
    Due to storage constraints, the various parts of the swap are spread 
    out between the three different mappings above: swaps, assets, 
    and premiums.
    */
    function setup(
        uint expectedAssetEscrow,
        uint expectedPremiumEscrow,
        address payable assetEscrower,
        address payable premiumEscrower,
        address assetAddress,
        bytes32 hashLock,
        uint startTime,
        bool firstAssetEscrow,
        uint delta
    )
        public 
        payable 
        canSetup(hashLock) 
    {
        //TODO
        // mapping(bytes32 => Swap) public swaps;
        swaps[hashLock] = Swap({
            assetEscrower: assetEscrower,
            premiumEscrower: premiumEscrower,
            hashLock: hashLock,
            assetAddress: assetAddress
        });

        // mapping(bytes32 => Asset) public assets;
        assets[hashLock] = Asset({
            expected: expectedAssetEscrow,
            current: 0,
             //The expression checks if firstAssetEscrow is true. If it is, deadline will be assigned the value startTime + 3 * delta. 
             // If firstAssetEscrow is false, deadline will be assigned the value startTime + 4 * delta.
            deadline: firstAssetEscrow ? startTime + 3*delta : startTime + 4*delta,
            timeout: firstAssetEscrow ? startTime + 6*delta : startTime + 5*delta
        });
         
         // mapping(bytes32 => Premium) public premiums;
         premiums[hashLock] = Premium({
            expected: expectedPremiumEscrow,
            current: 0,
            deadline: firstAssetEscrow ? startTime + 2*delta : startTime + 1*delta
         });

         emit SetUp(assetEscrower, premiumEscrower, expectedPremiumEscrow, expectedAssetEscrow, startTime, 
            firstAssetEscrow ? startTime + 2*delta : startTime + 1*delta, 
            firstAssetEscrow ? startTime + 3*delta : startTime + 4*delta,
            firstAssetEscrow ? startTime + 6*delta : startTime + 5*delta);

    }

    /**
    The premium escrower has to escrow their premium for the protocol to succeed.
    */
    function escrowPremium(bytes32 hashLock)
        public
        payable
        canEscrowPremium(hashLock)
    {
       //TODO
       //  A reference to the ERC20 token contract 
       ERC20 amount = ERC20(swaps[hashLock].assetAddress);
       uint escrow_amount = premiums[hashLock].expected - premiums[hashLock].current;

       // transferFrom(address _from, address _to, uint256 _value): 
       // A function that allows transferring tokens from one address to another on behalf of the token owner, 
       // given that the transaction is approved.

       // msg: The msg object represents the current message or transaction that is calling a function within a contract.
       // The this object represents the current instance of the contract in which the code is being executed.
       require(amount.transferFrom(msg.sender, address(this), escrow_amount), "fail to transfer!");

       premiums[hashLock].current += escrow_amount;

       // In Solidity, emit is a keyword used to trigger an event.
       emit PremiumEscrowed(
                msg.sender,
                escrow_amount,
                swaps[hashLock].premiumEscrower,
                address(this),
                premiums[hashLock].current,
                assets[hashLock].current
        );
    }

    /**
    The asset escrower has to escrow their premium for the protocol to succeed
    */
    function escrowAsset(bytes32 hashLock) 
        public 
        payable 
        canEscrowAsset(hashLock) 
    {
        //TODO
        ERC20 amount = ERC20(swaps[hashLock].assetAddress);
        uint escrow_amount = assets[hashLock].expected - assets[hashLock].current;

        require(amount.transferFrom(msg.sender, address(this), escrow_amount), "fail to transfer!");

        assets[hashLock].current += escrow_amount;

        emit AssetEscrowed(msg.sender, escrow_amount, swaps[hashLock].assetEscrower, address(this), premiums[hashLock].current, assets[hashLock].current);

    }

    /**
    redeemAsset redeems the asset for the new owner
    */
    function redeemAsset(bytes32 preimage, bytes32 hashLock) 
        public 
        canRedeemAsset(preimage, hashLock) 
    {
        //TODO
        ERC20 amount = ERC20(swaps[hashLock].assetAddress);
        uint current_amount = assets[hashLock].current;

        // transfer(address _to, uint256 _value): 
        // A function that transfers a specified amount of tokens from the caller's address to another address.
        require(amount.transfer(swaps[hashLock].premiumEscrower, current_amount), "fail to transfer!");

        assets[hashLock].current = 0;

        emit AssetRedeemed(msg.sender, current_amount, address(this), swaps[hashLock].premiumEscrower, premiums[hashLock].current, assets[hashLock].current);
    }

    /**
    refundPremium refunds the premiumEscrower's premium should the swap succeed
    */
    function refundPremium(bytes32 hashLock) 
        public 
        canRefundPremium(hashLock)
    {
        //TODO
        ERC20 amount = ERC20(swaps[hashLock].assetAddress);
        uint refund_amount = premiums[hashLock].current;
        
        require(amount.transfer(msg.sender, refund_amount), "fail to transfer!");

        premiums[hashLock].current = premiums[hashLock].current - refund_amount;

        emit PremiumRefunded(msg.sender, refund_amount, address(this), msg.sender, premiums[hashLock].current, assets[hashLock].current);
    }

    /**
    refundAsset refunds the asset to its original owner should the swap fail
    */
    function refundAsset(bytes32 hashLock) 
        public 
        canRefundAsset(hashLock) 
    {
       //TODO
       ERC20 amount = ERC20(swaps[hashLock].assetAddress);
       uint refund_amount = assets[hashLock].current;
       
       require(amount.transfer(msg.sender, refund_amount), "fail to transfer!");

       assets[hashLock].current = 0;

       emit AssetRefunded(msg.sender, refund_amount, address(this), msg.sender, premiums[hashLock].current, assets[hashLock].current);
    }

    /**
    redeemPremium allows a party to redeem the counterparty's premium should the swap fail
    */
    function redeemPremium(bytes32 hashLock) 
        public 
        canRedeemPremium(hashLock)
    {
        //TODO
       ERC20 amount = ERC20(swaps[hashLock].assetAddress);
       uint redeem_amount = premiums[hashLock].current;
       
       require(amount.transfer(swaps[hashLock].assetEscrower, redeem_amount), "fail to transfer!");

       premiums[hashLock].current = 0;

       emit PremiumRedeemed(msg.sender, redeem_amount, address(this), swaps[hashLock].assetEscrower, premiums[hashLock].current, assets[hashLock].current);
    }
}
