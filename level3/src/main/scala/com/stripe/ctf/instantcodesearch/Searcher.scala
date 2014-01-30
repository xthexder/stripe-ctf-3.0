package com.stripe.ctf.instantcodesearch

import java.io._
import java.nio.file._

import com.twitter.concurrent.Broker
import sys.process.stringSeqToProcess
import scala.language.postfixOps

abstract class SearchResult
case class Match(output: String) extends SearchResult
case class Done() extends SearchResult

class Searcher(indexPath : String)  {
  val index : Index = readIndex(indexPath)
  val root = FileSystems.getDefault().getPath(index.path)

  def search(needle : String, b : Broker[SearchResult]) = {
    var results = (Seq("bash", "-c", "cd " + root + " && grep -n -H \"" + needle + "\" " + index.files + " | cut -s -d: -f1,2 && echo hi")!!).split("\n")
    // Echo "hi" because scala will hang waiting on output if nothing is found
    for (result <- results) {
      if (result.length() > 2) b !! new Match(result.substring(2))
    }

    b !! new Done()
  }

  def readIndex(path: String) : Index = {
    new ObjectInputStream(new FileInputStream(new File(path))).readObject.asInstanceOf[Index]
  }
}
