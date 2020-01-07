import React, {Component} from 'react'
import './App.css'
import Header from './components/Header.js'
import Login from './components/Login.js'

class App extends Component {
  render() {                                                                                                                                                             
	  return (
      <div className="container">
       <Login />
      </div>
    );
  }
}

export default App;
