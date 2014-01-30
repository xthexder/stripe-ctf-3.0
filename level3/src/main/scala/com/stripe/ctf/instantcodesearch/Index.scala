package com.stripe.ctf.instantcodesearch

import java.io._
import scala.collection.mutable.HashMap
import scala.collection.Set

class Index(repoPath: String) extends Serializable {
  var files = ""

  def path() = repoPath

  def addFile(file: String) {
    if (files.length() > 0) {
      files += " ./" + file
    } else {
      files = "./" + file
    }
  }

  def write(out: File) {
    val stream = new FileOutputStream(out)
    write(stream)
    stream.close
  }

  def write(out: OutputStream) {
    val w = new ObjectOutputStream(out)
    w.writeObject(this)
    w.close
  }
}

