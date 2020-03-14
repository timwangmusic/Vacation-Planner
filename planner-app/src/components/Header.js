import React , { Component } from "react"

class Header extends Component {
    render() {
        return <header className="headerCommon">Vacation Plans for {this.props.TravelDestination}</header>
    }
}

export default Header
