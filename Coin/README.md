## Components
Coin consists of several main components. Here's a quick overview:

1. **Blockchain**
    - See previous project.
2. **Miner**
    - The Miner handles our proof of work (POW) consensus mechanism. It's in charge of finding a winning nonce for the blocks it forms. The miner keeps track of a Transaction Pool, which contains transactions passed to it by the node. Off to the races!
3. **Node**
    - The node handles all top level logic. Among its various duties, it validates incoming blocks (using our blockchain), broadcasts transaction requests from the wallet, and tells the miner (if it has one) when to stop and start mining. Whenever it sees a valid block that appends to its chain, the node should tell its miner to get busy on a new block.
4. **Wallet**
    - The wallet keeps track of our coins and creates transactions, which it passes along to its node. The node can then broadcast these transactions to the network. Miners of nodes that hear about these transactions will add them to their transaction pools. Given a large enough fee (or enough space on a block), the miner will include the transaction in their block.
5. **Server**
    - If we wanted to, we could start a server and run our very own cryptocurrency out of Brown. We're skeptical this would be a good idea, since there are plenty of bugs lurking around in the code base. But we still could! **You don't need to worry about servers for this assignment**, but we still wanted to mention it here.