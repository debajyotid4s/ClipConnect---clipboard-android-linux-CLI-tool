package main
import ("fmt";"os";"os/exec")
func main(){
  c := exec.Command("tail","-n","10","/tmp/cc-home/clipconnect/daemon.log")
  c.Stdin = os.Stdin
  c.Stdout = os.Stdout
  c.Stderr = os.Stderr
  if err := c.Run(); err != nil { fmt.Println("ERR:", err) }
}
