Stripe CTF 3.0 Solutions
========================
Jacob Wirth (xthexder)  
[https://stripe-ctf.com/achievements/xthexder](https://web.archive.org/web/20140424194302/https://stripe-ctf.com/achievements/xthexder)  
[https://stripe-ctf.com/leaderboard](https://web.archive.org/web/20140210001038/https://stripe-ctf.com/leaderboard)

A copy of the original challenge prompts can be found here: https://github.com/ctfs/write-ups-2014/tree/master/stripe-ctf3

Level0
------
Final score: 158  
Final rank: 904

**Notable Changes**
* Used a ruby hash map to determine if a word was in the database
* Change code from gsub to scan (this probably didn't make a difference at all)
* Printing in loop instead of after

Level1
------
Final score: 50  
Final rank: 103 (Same as everyone else who didn't participate in the PvP round)  
Approximate hash rate: 500 kH/s on an Intel i7 960 @ 3.2GHz

**Notable Implementation Points**
* Written in Golang for mining on the CPU
* Used original bash script to run to go miner and commit coins
* 8 threads to run one per core
* Didn't get used for the PvP round, although it might have been able to mine a couple coins if I had tried submitting it ASAP

Level2
------
Final score: 140  
Final rank: 304

I found this level to be fairly annoying because it was tested really strangely. If you let all requests through, the backends will not fall over, and infact 1 of the 2 backends will be completely inactive because the sword program runs queries synchronously. This means you will get point deductions for servers being inactive no matter what.

**Notable Changes**
* Changed sword.js to output detailed logs about totals, not just correct
* Let a maximum of 4 queries through per IP
* Reject f requests were less than 25ms apart

Level3
------
Final score: 881  
Final rank: 165

I hate Scala so much...

**Notable Changes**
* During the indexing, make a list of all the files split into 3 groups, one for each shard
* Run grep on all the files for each shard
* Join output from each shard on the master node and respond
* Changed shard's output so it doesn't need to be parsed by the master

Level4
------
Final score: 17205  
Final rank: 1  
Average non-normalized score: 25000-30000 (2500-3000 queries)

I spent the most time on this level by far. I've included two solutions for this level, one that would be considered more "legitimate". The submittion scoring 17k+ is in the folder called `level5`.

Before you check out the code, I'll show you one of the best runs I got with the `level5` solution:

    remote: 2014/01/30 03:33:36 The allotted 30s have elapsed. Exiting!
    remote: > Finished running Trial 0
    remote: >
    remote: ------------------------------------------
    remote: >
    remote: > Your results are in!
    remote: >
    remote: > Trial 0 stats:
    remote: --------------------8<--------------------
    remote: Final stats (running with 5 nodes for 30s):
    remote: Bytes read: 0.00B
    remote: Bytes written: 0.00B
    remote: Connection attempts: 0
    remote: Correct queries: 3042
    remote:
    remote: Score breakdown:
    remote: 30420 points from queries
    remote: -0 points from network traffic
    remote:
    remote: Total:
    remote: 30420 points
    remote: -------------------->8--------------------

Notice anything fishy? No network traffic maybe?
After tons of iterations on this solutions, I ended up coming up with this extremely simple, yet very effective solution to level4.

**Timeline**  
The first thing I did when starting this problem is grab a copy of [goraft](https://github.com/goraft/raft) and the [raftd example](https://github.com/goraft/raftd). I noticed that the example code for this level was almost exactly the same as the raftd example, and it was a simple matter of connecting things together again.

I started with regular request forwarding, doing a POST to the leader node, and waiting for a response. This worked perfectly until Saturday when the scores were reset and the single point of failure tests were added. What ended up happening was that the query would be run, but the response would be lost in octopus. I tried a few things with forwarding through other nodes to get a response, and this worked to a degree, but wasn't perfect.

At this point I had started looking at using HTTP redirects to forward instead of proxying. Octopus [does't explicitly follow redirects](https://github.com/stripe-ctf/octopus/blob/master/harness/harness.go#L158), but that didn't stop me from trying. As it turns out, the underlying Go library does follow redirects on a POST by issuing a GET to the redirect URL. All I had to do to get it working was pass in the POST data as a GET parameter. This is roughly the point the code in the `level4` folder is from. There were of course some hoops to jump through to get redirects working with the absolute socket paths, but you can look at the code for that.

I spent a lot more time trying to optimize things and get the forwarding to work faster. I was fairly successful and ended up in ~5th place. I then had the idea for what `level5` turned into. "What if I send all the data through redirects?" Since the connections between nodes and octopus is "perfect", it would allow for perfect data transfer between nodes. I implemented this by redirecting through each of the nodes and running the query on each until the last one, which would return the result. This worked amazingly with the occational discontinuity error due to two different redirect queries overlapping eachother.

What I noticed in the process of this is that nodes are never turned off by the octopus "murder monkey". And infact if it had been turned on, my current solution wouldn't work, since a redirect would just stop at a dead node with no response. What this allowed me to do however, was use a single node for all the database queries. The solution implemented in `level5` is exactly this: Every node but node0 redirects the query to node0, and node0 responds. This got me into 1st place briefly, but I ended up getting pushed down to 4th fairly quickly.

Now it's time for optimizations. The first thing I did to optimize my code was to rip out the leftover raft library, hardcoding all the node names and taking out the whole cluster join process. After that it was the sql library. My current solution involves a single regex to parse the very similar queries and grab the specific fiels for doing my own in-memory managed user map. I ended up getting this up to 60k queries/s during my benchmarking. I also took out any `fmt.Sprintf`'s I had and replaced them with string joins.

All that happened after this was me researching the marking process. Your non-normalized score is just `total queries * 10 - network traffic`. Your score is then compared to the benchmark, and your normalized score works out to `you / benchmark * 100`. I created a 1-line bash script that just ran `git push` over and over again, eventually getting a submittion where my score was high, and the benchmark score was really low, causing a high ratio:
    remote: > Your normalized score on Trial 0 is 17094. (Our benchmark scored 770 [queries] - 615 [network] = 154 on this test case.)

150 was about the lowest I saw the benchmark score, but sometimes it would get as high as 4000, and my normalized score would be in the 100s.

Now that you know how I did it, I hope you enjoyed the 3rd Stripe CTF, I certainly did (as tense as it was being on the leaderboard for so long).

**Summary of Changes**
* Implemented [raftd](https://github.com/goraft/raftd) into the Stripe sample code
* Set up unix socket connections by setting the http transport
* Forwarded connections from followers using proxying (`level4`) or redirects (`level5`)
`level5` solution only:
* Ripped out raft completely
* Hardcoded all nodes to redirect queries to node0
* Changed sqlite to a regex and a go struct
* Ripped out [mux](https://github.com/gorilla/mux) and replaced it with a much simpler handler
